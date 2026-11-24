package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bongdo/ink/internal/driver"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type positionArgs struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type diagnoseArgs struct {
	Path string `json:"path"`
	Fast bool   `json:"fast"`
}

const protocolVersion = "2025-06-18"

var tools = []tool{
	{
		Name:        "diagnose",
		Description: "Compile the Ink project and return ink check diagnostics as JSON (empty array is clean). fast=true skips the generated-Go typecheck and go vet.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"optional absolute path to filter diagnostics"},"fast":{"type":"boolean","description":"sema only, skips Go typecheck and vet"}}}`),
	},
	{
		Name:        "hover",
		Description: "Type and signature at a position: variables, functions, fields, inferred Go types.",
		InputSchema: json.RawMessage(`{"type":"object","required":["path","line","column"],"properties":{"path":{"type":"string"},"line":{"type":"integer","description":"1-based"},"column":{"type":"integer","description":"1-based"}}}`),
	},
	{
		Name:        "definition",
		Description: "Jump to the declaration of the name at a position.",
		InputSchema: json.RawMessage(`{"type":"object","required":["path","line","column"],"properties":{"path":{"type":"string"},"line":{"type":"integer","description":"1-based"},"column":{"type":"integer","description":"1-based"}}}`),
	},
	{
		Name:        "references",
		Description: "All references to the declaration at a position.",
		InputSchema: json.RawMessage(`{"type":"object","required":["path","line","column"],"properties":{"path":{"type":"string"},"line":{"type":"integer","description":"1-based"},"column":{"type":"integer","description":"1-based"}}}`),
	},
	{
		Name:        "complete",
		Description: "Completion candidates at a position: locals, declarations, fields and methods after a dot.",
		InputSchema: json.RawMessage(`{"type":"object","required":["path","line","column"],"properties":{"path":{"type":"string"},"line":{"type":"integer","description":"1-based"},"column":{"type":"integer","description":"1-based"}}}`),
	},
	{
		Name:        "symbols",
		Description: "Declarations in one file: types, functions, methods, tests.",
		InputSchema: json.RawMessage(`{"type":"object","required":["path"],"properties":{"path":{"type":"string"}}}`),
	},
	{
		Name:        "rename",
		Description: "Edits for renaming the symbol at a position; rejected when it would break or rebind. Returns edits, does not apply them.",
		InputSchema: json.RawMessage(`{"type":"object","required":["path","line","column","newName"],"properties":{"path":{"type":"string"},"line":{"type":"integer","description":"1-based"},"column":{"type":"integer","description":"1-based"},"newName":{"type":"string"}}}`),
	},
	{
		Name:        "quickfix",
		Description: "Quick fixes for the diagnostic at a position: make to T[](n), for (cond) to for (; cond; ), import and const repairs. Returns edits, does not apply them.",
		InputSchema: json.RawMessage(`{"type":"object","required":["path","line","column"],"properties":{"path":{"type":"string"},"line":{"type":"integer","description":"1-based"},"column":{"type":"integer","description":"1-based"}}}`),
	},
}

type renameArgs struct {
	positionArgs
	NewName string `json:"newName"`
}

func Run(stdin io.Reader, stdout io.Writer, root string) error {
	sec := &session{root: root}
	defer sec.close()
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 1<<20), 16<<20)
	enc := json.NewEncoder(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		if req.ID == nil {
			continue
		}
		resp := response{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			resp.Result = map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "ink", "version": driver.Version},
			}
		case "ping":
			resp.Result = map[string]any{}
		case "tools/list":
			resp.Result = map[string]any{"tools": tools}
		case "tools/call":
			result, err := sec.call(req.Params)
			if err != nil {
				resp.Error = &rpcError{Code: -32603, Message: err.Error()}
			} else {
				resp.Result = result
			}
		default:
			resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

type session struct {
	root      string
	lsp       *lspClient
	diagKey   string
	diagCache []driver.JSONError
}

func (s *session) close() {
	if s.lsp != nil {
		s.lsp.close()
	}
}

func (s *session) call(raw json.RawMessage) (any, error) {
	var p callParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	switch p.Name {
	case "diagnose":
		var a diagnoseArgs
		_ = json.Unmarshal(p.Arguments, &a)
		text, err := s.diagnose(a)
		if err != nil {
			return nil, err
		}
		return textResult(text), nil
	case "rename":
		var a renameArgs
		if err := json.Unmarshal(p.Arguments, &a); err != nil {
			return nil, err
		}
		if a.Path == "" || a.Line < 1 || a.Column < 1 || a.NewName == "" {
			return nil, errors.New("path, line, column and newName are required")
		}
		text, err := s.rename(a)
		if err != nil {
			return nil, err
		}
		return textResult(text), nil
	case "quickfix":
		var a positionArgs
		if err := json.Unmarshal(p.Arguments, &a); err != nil {
			return nil, err
		}
		text, err := s.quickfix(a)
		if err != nil {
			return nil, err
		}
		return textResult(text), nil
	case "symbols":
		var a positionArgs
		if err := json.Unmarshal(p.Arguments, &a); err != nil {
			return nil, err
		}
		text, err := s.symbols(a.Path)
		if err != nil {
			return nil, err
		}
		return textResult(text), nil
	case "hover", "definition", "references", "complete":
		var a positionArgs
		if err := json.Unmarshal(p.Arguments, &a); err != nil {
			return nil, err
		}
		if a.Path == "" || a.Line < 1 || a.Column < 1 {
			return nil, errors.New("path, line and column are required (1-based)")
		}
		text, err := s.position(p.Name, a)
		if err != nil {
			return nil, err
		}
		return textResult(text), nil
	}
	return nil, fmt.Errorf("unknown tool %q", p.Name)
}

func (s *session) rename(a renameArgs) (string, error) {
	if s.lsp == nil {
		c, err := startLSP(s.root)
		if err != nil {
			return "", err
		}
		s.lsp = c
	}
	uri, err := s.lsp.openFile(a.Path)
	if err != nil {
		return "", err
	}
	raw, err := s.lsp.request("textDocument/rename", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": a.Line - 1, "character": a.Column - 1},
		"newName":      a.NewName,
	})
	if err != nil {
		return "", fmt.Errorf("rename rejected: %v", err)
	}
	return formatEdits(raw), nil
}

func (s *session) allDiagnostics(fast bool) ([]driver.JSONError, error) {
	key := fingerprint(s.root)
	if !fast && s.diagCache != nil && key == s.diagKey {
		return s.diagCache, nil
	}
	p, err := driver.Open(s.root)
	if err != nil {
		return nil, err
	}
	if err := p.Check(driver.CheckOptions{Tidy: true, Typecheck: !fast, Fast: fast}); err != nil {
		return nil, err
	}
	diags := p.Diagnostics()
	if !fast {
		s.diagKey, s.diagCache = key, diags
	}
	return diags, nil
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
		if !strings.HasSuffix(name, ".ink") && name != "go.mod" && name != "go.sum" {
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

func (s *session) quickfix(a positionArgs) (string, error) {
	diags, err := s.diagnosticsAt(a)
	if err != nil {
		return "", err
	}
	if len(diags) == 0 {
		return "no diagnostics at this position", nil
	}
	if s.lsp == nil {
		c, err := startLSP(s.root)
		if err != nil {
			return "", err
		}
		s.lsp = c
	}
	uri, err := s.lsp.openFile(a.Path)
	if err != nil {
		return "", err
	}
	raw, err := s.lsp.request("textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"range":        diags[0]["range"],
		"context":      map[string]any{"diagnostics": diags},
	})
	if err != nil {
		return "", err
	}
	var actions []struct {
		Title string `json:"title"`
		Edit  struct {
			Changes map[string][]struct {
				NewText string
				Range   struct {
					Start struct{ Line, Character int }
				}
			} `json:"changes"`
		} `json:"edit"`
	}
	if err := json.Unmarshal(raw, &actions); err != nil || len(actions) == 0 {
		hint := ""
		for _, d := range diags {
			if h, ok := d["hint"].(string); ok && h != "" {
				hint = h
			}
		}
		if hint != "" {
			return "no automatic fix here; hint: " + hint, nil
		}
		return "no quick fixes", nil
	}
	var b strings.Builder
	for _, action := range actions {
		b.WriteString(action.Title)
		for uri, edits := range action.Edit.Changes {
			path := strings.TrimPrefix(uri, "file://")
			for _, e := range edits {
				b.WriteString("\n  " + path + ":" + strconv.Itoa(e.Range.Start.Line+1) + ":" + strconv.Itoa(e.Range.Start.Character+1) + " -> " + e.NewText)
			}
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String()), nil
}

func (s *session) diagnosticsAt(a positionArgs) ([]map[string]any, error) {
	p, err := driver.Open(s.root)
	if err != nil {
		return nil, err
	}
	if err := p.Check(driver.CheckOptions{Tidy: true, Typecheck: true}); err != nil {
		return nil, err
	}
	abs, _ := filepath.Abs(a.Path)
	var out []map[string]any
	for _, d := range p.Diagnostics() {
		if d.File != abs {
			continue
		}
		if a.Line < d.Line || a.Line > d.EndLine {
			continue
		}
		out = append(out, map[string]any{
			"range": map[string]any{
				"start": map[string]any{"line": d.Line - 1, "character": d.Col - 1},
				"end":   map[string]any{"line": d.EndLine - 1, "character": d.EndCol - 1},
			},
			"severity": 1,
			"code":     d.Code,
			"message":  d.Message,
			"hint":     d.Hint,
			"source":   "ink",
		})
	}
	return out, nil
}

func (s *session) symbols(path string) (string, error) {
	if s.lsp == nil {
		c, err := startLSP(s.root)
		if err != nil {
			return "", err
		}
		s.lsp = c
	}
	uri, err := s.lsp.openFile(path)
	if err != nil {
		return "", err
	}
	raw, err := s.lsp.request("textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": uri}})
	if err != nil {
		return "", err
	}
	type mcpSymbol struct {
		Name     string                             `json:"name"`
		Kind     int                                `json:"kind"`
		Range    struct{ Start struct{ Line int } } `json:"range"`
		Children []mcpSymbol                        `json:"children"`
	}
	var syms []mcpSymbol
	if err := json.Unmarshal(raw, &syms); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, sym := range syms {
		fmt.Fprintf(&b, "%s %s:%d\n", sym.Name, path, sym.Range.Start.Line+1)
		for _, child := range sym.Children {
			fmt.Fprintf(&b, "  %s %s:%d\n", child.Name, path, child.Range.Start.Line+1)
		}
	}
	if b.Len() == 0 {
		return "no symbols", nil
	}
	return strings.TrimSpace(b.String()), nil
}

func formatEdits(raw json.RawMessage) string {
	var edit struct {
		Changes map[string][]struct {
			Range struct {
				Start struct{ Line, Character int }
			}
			NewText string `json:"newText"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(raw, &edit); err != nil {
		return "no edits"
	}
	var b strings.Builder
	for uri, edits := range edit.Changes {
		path := strings.TrimPrefix(uri, "file://")
		for _, e := range edits {
			fmt.Fprintf(&b, "%s:%d:%d -> %s\n", path, e.Range.Start.Line+1, e.Range.Start.Character+1, e.NewText)
		}
	}
	if b.Len() == 0 {
		return "no edits"
	}
	return strings.TrimSpace(b.String())
}

func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func (s *session) diagnose(a diagnoseArgs) (string, error) {
	diags, err := s.allDiagnostics(a.Fast)
	if err != nil {
		return "", err
	}
	path := a.Path
	if path != "" {
		abs, err := filepath.Abs(path)
		if err == nil {
			filtered := diags[:0]
			for _, d := range diags {
				if d.File == abs || strings.HasSuffix(d.File, string(filepath.Separator)+path) {
					filtered = append(filtered, d)
				}
			}
			diags = filtered
		}
	}
	data, err := json.MarshalIndent(diags, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *session) position(tool string, a positionArgs) (string, error) {
	if s.lsp == nil {
		c, err := startLSP(s.root)
		if err != nil {
			return "", err
		}
		s.lsp = c
	}
	uri, err := s.lsp.openFile(a.Path)
	if err != nil {
		return "", err
	}
	pos := map[string]any{"line": a.Line - 1, "character": a.Column - 1}
	doc := map[string]any{"uri": uri}
	switch tool {
	case "hover":
		raw, err := s.lsp.request("textDocument/hover", map[string]any{"textDocument": doc, "position": pos})
		if err != nil {
			return "", err
		}
		var h struct {
			Contents struct {
				Value string `json:"value"`
			} `json:"contents"`
		}
		if json.Unmarshal(raw, &h) != nil || h.Contents.Value == "" {
			return "no hover information", nil
		}
		return h.Contents.Value, nil
	case "definition":
		raw, err := s.lsp.request("textDocument/definition", map[string]any{"textDocument": doc, "position": pos})
		if err != nil {
			return "", err
		}
		return formatLocations(raw), nil
	case "references":
		raw, err := s.lsp.request("textDocument/references", map[string]any{"textDocument": doc, "position": pos, "context": map[string]any{"includeDeclaration": true}})
		if err != nil {
			return "", err
		}
		return formatLocations(raw), nil
	case "complete":
		raw, err := s.lsp.request("textDocument/completion", map[string]any{"textDocument": doc, "position": pos})
		if err != nil {
			return "", err
		}
		return formatCompletions(raw), nil
	}
	return "", fmt.Errorf("unknown tool %q", tool)
}

type location struct {
	URI   string `json:"uri"`
	Range struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
	} `json:"range"`
}

func formatLocations(raw json.RawMessage) string {
	var list []location
	if json.Unmarshal(raw, &list) != nil || len(list) == 0 {
		var one location
		if json.Unmarshal(raw, &one) == nil && one.URI != "" {
			list = []location{one}
		}
	}
	if len(list) == 0 {
		return "no definition"
	}
	var b strings.Builder
	for _, l := range list {
		b.WriteString(locText(l))
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func locText(l location) string {
	path := strings.TrimPrefix(l.URI, "file://")
	return path + ":" + strconv.Itoa(l.Range.Start.Line+1) + ":" + strconv.Itoa(l.Range.Start.Character+1)
}

func formatCompletions(raw json.RawMessage) string {
	var list struct {
		Items []struct {
			Label  string `json:"label"`
			Detail string `json:"detail"`
			Kind   int    `json:"kind"`
		} `json:"items"`
	}
	if json.Unmarshal(raw, &list) != nil || len(list.Items) == 0 {
		return "no completions"
	}
	var b strings.Builder
	for _, item := range list.Items {
		b.WriteString(item.Label)
		if item.Detail != "" {
			b.WriteString(" — " + item.Detail)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
