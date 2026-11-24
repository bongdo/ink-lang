package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bongdo/ink/internal/config"
	"github.com/bongdo/ink/internal/driver"
	"github.com/bongdo/ink/internal/sema"
)

type mcpServer struct {
	root   string
	stderr io.Writer
	srv    *server
	last   *analysis
	fp     string
	exit   bool
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func RunMCP(in io.Reader, out io.Writer, stderr io.Writer, root string) error {
	m := &mcpServer{root: root, stderr: stderr}
	r := bufio.NewReaderSize(in, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			if reply := m.handle(trimmed); reply != nil {
				if _, werr := out.Write(append(reply, '\n')); werr != nil {
					return werr
				}
			}
		}
		if m.exit {
			return nil
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func (m *mcpServer) handle(line []byte) []byte {
	var msg rpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return rpcError(nil, -32700, "parse error")
	}
	if len(msg.ID) == 0 {
		if msg.Method == "exit" {
			m.exit = true
		}
		return nil
	}
	switch msg.Method {
	case "initialize":
		return rpcResult(msg.ID, map[string]any{
			"protocolVersion": protocolVersion(msg.Params),
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "ink", "version": driver.Version},
		})
	case "ping":
		return rpcResult(msg.ID, map[string]any{})
	case "tools/list":
		return rpcResult(msg.ID, map[string]any{"tools": mcpTools()})
	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(msg.Params, &call); err != nil {
			return rpcError(msg.ID, -32602, "bad tools/call params")
		}
		result, err := m.call(call.Name, call.Arguments)
		if err != nil {
			return rpcResult(msg.ID, map[string]any{
				"content": []any{map[string]any{"type": "text", "text": err.Error()}},
				"isError": true,
			})
		}
		return rpcResult(msg.ID, map[string]any{
			"content": []any{map[string]any{"type": "text", "text": result}},
		})
	}
	return rpcError(msg.ID, -32601, "method not found: "+msg.Method)
}

func protocolVersion(params json.RawMessage) string {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(params) > 0 && json.Unmarshal(params, &p) == nil && p.ProtocolVersion != "" {
		return p.ProtocolVersion
	}
	return "2024-11-05"
}

func rpcResult(id json.RawMessage, result any) []byte {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result})
	if err != nil {
		return rpcError(id, -32603, err.Error())
	}
	return body
}

func rpcError(id json.RawMessage, code int, message string) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error":   map[string]any{"code": code, "message": message},
	})
	return body
}

func mcpTools() []map[string]any {
	pos := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":   map[string]any{"type": "string", "description": "absolute or project-relative .ink file"},
			"line":   map[string]any{"type": "integer", "description": "1-based line"},
			"column": map[string]any{"type": "integer", "description": "1-based column"},
		},
		"required": []string{"path", "line", "column"},
	}
	return []map[string]any{
		{"name": "diagnose", "description": "Compiler diagnostics for the whole project or one file: path, line, column, endLine, endCol, severity, code, message, hint. Same output as ink check --json.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "optional .ink file; omit for the whole project"},
			}}},
		{"name": "hover", "description": "Type and signature at a position, including the inferred Go type for Go calls.", "inputSchema": pos},
		{"name": "definition", "description": "Jump target for the identifier at a position.", "inputSchema": pos},
		{"name": "references", "description": "All project references to the declaration at a position.", "inputSchema": pos},
		{"name": "complete", "description": "Completions at a position; after a dot this lists fields and methods.", "inputSchema": pos},
		{"name": "symbols", "description": "Document symbols of one file: types, functions, methods, tests.", "inputSchema": map[string]any{
			"type": "object", "properties": map[string]any{
				"path": map[string]any{"type": "string"},
			}, "required": []string{"path"},
		}},
		{"name": "rename", "description": "Renaming edits for the symbol at a position; rejected when it would break or rebind. Edits are returned, not applied.", "inputSchema": map[string]any{
			"type": "object", "properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"line":    map[string]any{"type": "integer"},
				"column":  map[string]any{"type": "integer"},
				"newName": map[string]any{"type": "string"},
			}, "required": []string{"path", "line", "column", "newName"},
		}},
		{"name": "quickfix", "description": "Quick fixes for the diagnostic at a position; edits are returned, not applied.", "inputSchema": map[string]any{
			"type": "object", "properties": map[string]any{
				"path":   map[string]any{"type": "string"},
				"line":   map[string]any{"type": "integer"},
				"column": map[string]any{"type": "integer"},
			}, "required": []string{"path", "line", "column"},
		}},
	}
}

type mcpPos struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	NewName string `json:"newName"`
}

func (p mcpPos) lspPosition() Position {
	line, col := p.Line-1, p.Column-1
	if line < 0 {
		line = 0
	}
	if col < 0 {
		col = 0
	}
	return Position{Line: line, Character: col}
}

func (m *mcpServer) call(name string, args json.RawMessage) (string, error) {
	switch name {
	case "diagnose":
		return m.diagnose(args)
	}
	var p mcpPos
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("bad arguments: %v", err)
	}
	path, err := m.resolve(p.Path)
	if err != nil {
		return "", err
	}
	if name == "symbols" {
		a := m.analysis()
		_ = a
		res, err := m.documentSymbols(path)
		if err != nil {
			return "", err
		}
		return jsonText(res)
	}
	if err := m.ensure(path, p.Line, p.Column); err != nil {
		return "", err
	}
	uri := pathToURI(path)
	switch name {
	case "hover":
		res, err := m.srv.hover(TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: p.lspPosition()})
		if err != nil {
			return "", err
		}
		if res == nil {
			return "", nil
		}
		hover, ok := res.(*Hover)
		if !ok || hover == nil {
			return jsonText(res)
		}
		return hover.Contents.Value, nil
	case "definition":
		res, err := m.srv.definition(TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: p.lspPosition()})
		return jsonTextResult(res, err)
	case "references":
		res, err := m.srv.references(ReferenceParams{
			TextDocumentPositionParams: TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: p.lspPosition()},
			Context:                    ReferenceContext{IncludeDeclaration: true},
		})
		return jsonTextResult(res, err)
	case "complete":
		res, err := m.srv.completion(CompletionParams{TextDocumentPositionParams: TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: p.lspPosition()}})
		if err != nil {
			return "", err
		}
		list, ok := res.(CompletionList)
		if !ok {
			return jsonText(res)
		}
		items := make([]map[string]any, 0, len(list.Items))
		for _, item := range list.Items {
			entry := map[string]any{"label": item.Label, "kind": item.Kind}
			if item.Detail != "" {
				entry["detail"] = item.Detail
			}
			items = append(items, entry)
		}
		return jsonText(items)
	case "rename":
		if p.NewName == "" {
			return "", errors.New("newName is required")
		}
		res, err := m.srv.rename(RenameParams{
			TextDocumentPositionParams: TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: p.lspPosition()},
			NewName:                    p.NewName,
		})
		return jsonTextResult(res, err)
	case "quickfix":
		return m.quickfix(path, uri, p)
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (m *mcpServer) diagnose(args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal(args, &a)
	diags, err := m.projectDiagnostics()
	if err != nil {
		return "", err
	}
	if a.Path == "" {
		return jsonText(diags)
	}
	path, err := m.resolve(a.Path)
	if err != nil {
		return "", err
	}
	filtered := make([]driver.JSONError, 0, len(diags))
	for _, d := range diags {
		if sameFile(d.File, path) {
			filtered = append(filtered, d)
		}
	}
	return jsonText(filtered)
}

func (m *mcpServer) projectDiagnostics() ([]driver.JSONError, error) {
	p, err := driver.Open(m.root)
	if err != nil {
		return nil, err
	}
	if err := p.Check(driver.CheckOptions{Tidy: true, Typecheck: true}); err != nil {
		return nil, err
	}
	return p.Diagnostics(), nil
}

func (m *mcpServer) quickfix(path, uri string, p mcpPos) (string, error) {
	a := m.last
	if a == nil {
		return "", errors.New("no analysis")
	}
	file := a.files[path]
	if file == nil {
		return "", fmt.Errorf("file not in project: %s", path)
	}
	pos := p.lspPosition()
	var diags []Diagnostic
	for _, d := range a.diags[path] {
		if positionIn(d.Range, pos) {
			diags = append(diags, d)
		}
	}
	if len(diags) == 0 {
		return "[]", nil
	}
	res, err := m.srv.codeActions(CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        diags[0].Range,
		Context:      CodeActionContext{Diagnostics: diags},
	})
	if err != nil {
		return "", err
	}
	return jsonText(res)
}

func positionIn(r Range, p Position) bool {
	if p.Line < r.Start.Line || p.Line > r.End.Line {
		return false
	}
	if p.Line == r.Start.Line && p.Character < r.Start.Character {
		return false
	}
	if p.Line == r.End.Line && p.Character > r.End.Character {
		return false
	}
	return true
}

func (m *mcpServer) documentSymbols(path string) (any, error) {
	a := m.last
	if a == nil {
		return nil, errors.New("no analysis")
	}
	uri := pathToURI(path)
	res, err := m.srv.documentSymbols(DocumentSymbolParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (m *mcpServer) resolve(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.root, path)
	}
	path = filepath.Clean(path)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("cannot read %s: %v", path, err)
	}
	if !strings.HasSuffix(path, ".ink") {
		return "", fmt.Errorf("%s is not an .ink file", path)
	}
	return path, nil
}

func (m *mcpServer) ensure(path string, line, column int) error {
	if line < 1 || column < 1 {
		return errors.New("line and column are 1-based")
	}
	m.analysis()
	if m.last == nil {
		return errors.New("analysis failed")
	}
	if m.last.files[path] == nil {
		return fmt.Errorf("file not in project: %s", path)
	}
	return nil
}

func (m *mcpServer) analysis() *analysis {
	fp := fingerprint(m.root)
	if m.last != nil && fp == m.fp && m.srv != nil {
		return m.last
	}
	module, _ := config.ReadModule(m.root)
	srv := &server{
		root:   m.root,
		module: module,
		docs:   map[string]*document{},
		cache:  map[string]*sema.GoPkg{},
	}
	a := srv.analyze(map[string]*document{}, m.root, module, false, 0)
	srv.last = a
	m.srv = srv
	m.last = a
	m.fp = fp
	return a
}

func fingerprint(root string) string {
	var b strings.Builder
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".ink") && name != "go.mod" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(&b, "%s:%d:%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return b.String()
}

func jsonText(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func jsonTextResult(v any, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return jsonText(v)
}

func sameFile(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return a == b
	}
	return filepath.Clean(aa) == filepath.Clean(bb)
}
