package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
}

const protocolVersion = "2025-06-18"

var tools = []tool{
	{
		Name:        "diagnose",
		Description: "Compile the Ink project and return ink check diagnostics as JSON (empty array is clean).",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"optional absolute path to filter diagnostics"}}}`),
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
	root string
	lsp  *lspClient
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
		text, err := s.diagnose(a.Path)
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

func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func (s *session) diagnose(path string) (string, error) {
	p, err := driver.Open(s.root)
	if err != nil {
		return "", err
	}
	if err := p.Check(driver.CheckOptions{Tidy: true, Typecheck: true}); err != nil {
		return "", err
	}
	diags := p.Diagnostics()
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
