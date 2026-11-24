package lsp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/types"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/config"
	"github.com/bongdo/ink/internal/diag"
	"github.com/bongdo/ink/internal/format"
	"github.com/bongdo/ink/internal/parser"
	"github.com/bongdo/ink/internal/sema"
	"github.com/bongdo/ink/internal/token"
	"github.com/bongdo/ink/internal/typecheck"
)

const (
	serverName           = "ink"
	serverVersion        = "0.2.0"
	textDocumentSyncFull = 1
	typecheckDelay       = 150 * time.Millisecond
)

const (
	ciKeyword  = 14
	ciFunction = 3
	ciVariable = 6
	ciClass    = 7
	ciModule   = 9
	ciEnum     = 13
	ciConstant = 21
	ciStruct   = 22
	ciField    = 5
)

const (
	dskClass      = 5
	dskMethod     = 6
	dskEnum       = 10
	dskInterface  = 11
	dskFunction   = 12
	dskVariable   = 13
	dskConstant   = 14
	dskEnumMember = 22
	dskStruct     = 23
)

var (
	errMethodNotFound = errors.New("method not found")
	errInvalidParams  = errors.New("invalid params")
)

var builtinTypes = []string{
	"void", "bool", "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64",
	"int", "uint", "f32", "f64", "str", "rune", "byte", "usize", "isize",
	"any", "error", "Duration", "Type",
}

var builtinFuncs = []string{
	"len", "cap", "append", "copy", "delete", "has", "panic", "assert", "assert_eq", "min", "max",
}

var keywords = []string{
	"use", "as", "fn", "type", "struct", "enum", "interface", "var", "const",
	"return", "if", "else", "for", "in", "match", "case", "go", "on",
	"limit", "defer", "break", "continue", "or", "fail", "arena", "test", "true", "false", "nil", "map",
}

type document struct {
	text    string
	version int
}

type fileInfo struct {
	path    string
	rel     string
	src     []byte
	file    *ast.File
	prog    *sema.Program
	version int
	open    bool
}

type analysis struct {
	files          map[string]*fileInfo
	nodeFile       map[ast.Node]string
	diags          map[string][]Diagnostic
	parsed         map[string][]*ast.File
	prog           *sema.Program
	root           string
	module         string
	asyncTypecheck bool
}

type server struct {
	conn      *Conn
	stderr    io.Writer
	rootArg   string
	root      string
	module    string
	docs      map[string]*document
	cache     map[string]*sema.GoPkg
	last      *analysis
	published map[string]bool
	info      *typecheck.Inferred
	infoGen   int

	mu     sync.Mutex
	gen    int
	timer  *time.Timer
	worker *queue
}

func Run(stdin io.Reader, stdout io.Writer, stderr io.Writer, root string) error {
	if stdin == nil {
		return errors.New("lsp: nil stdin")
	}
	if stdout == nil {
		return errors.New("lsp: nil stdout")
	}
	if stderr == nil {
		stderr = io.Discard
	}
	s := &server{
		conn:      NewConn(stdin, stdout),
		stderr:    stderr,
		rootArg:   root,
		docs:      make(map[string]*document),
		cache:     make(map[string]*sema.GoPkg),
		published: make(map[string]bool),
	}
	s.worker = newQueue(nil, func(gen int, result any) {
		s.publishAnalysis(gen, result)
	})
	return s.serve()
}

func (s *server) serve() error {
	defer s.stop()
	for {
		m, err := s.conn.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if m.Method == "" {
			continue
		}
		if len(m.ID) == 0 {
			if m.Method == "exit" {
				return nil
			}
			if err := s.handleNotify(m.Method, m.Params); err != nil {
				return err
			}
			continue
		}
		res, err := s.handleRequest(m.Method, m.Params)
		if err != nil {
			switch {
			case errors.Is(err, errInvalidParams):
				if werr := s.conn.ReplyError(m.ID, -32602, "invalid params"); werr != nil {
					return werr
				}
			case errors.Is(err, errMethodNotFound):
				if werr := s.conn.ReplyError(m.ID, -32601, "method not found: "+m.Method); werr != nil {
					return werr
				}
			default:
				s.logf("%v", err)
				if werr := s.conn.ReplyError(m.ID, -32603, err.Error()); werr != nil {
					return werr
				}
			}
			continue
		}
		if err := s.conn.Reply(m.ID, res); err != nil {
			return err
		}
		if m.Method == "initialize" {
			s.mu.Lock()
			err := s.reload(true)
			s.mu.Unlock()
			if err != nil {
				return err
			}
		}
	}
}

func (s *server) handleRequest(method string, params json.RawMessage) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch method {
	case "initialize":
		var p InitializeParams
		if len(params) > 0 {
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, errInvalidParams
			}
		}
		s.initialize(p)
		return InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync:           textDocumentSyncFull,
				HoverProvider:              true,
				DefinitionProvider:         true,
				ReferencesProvider:         true,
				RenameProvider:             true,
				CodeActionProvider:         true,
				CompletionProvider:         &CompletionOptions{TriggerCharacters: []string{".", ":"}},
				DocumentSymbolProvider:     true,
				WorkspaceSymbolProvider:    true,
				DocumentFormattingProvider: true,
				SignatureHelpProvider:      &SignatureHelpOptions{TriggerCharacters: []string{"(", ","}},
			},
			ServerInfo: ServerInfo{Name: serverName, Version: serverVersion},
		}, nil
	case "shutdown":
		return nil, nil
	case "textDocument/hover":
		var p TextDocumentPositionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.hover(p)
	case "textDocument/definition":
		var p TextDocumentPositionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.definition(p)
	case "textDocument/references":
		var p ReferenceParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.references(p)
	case "textDocument/prepareRename":
		var p TextDocumentPositionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.prepareRename(p)
	case "textDocument/rename":
		var p RenameParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.rename(p)
	case "textDocument/codeAction":
		var p CodeActionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.codeActions(p)
	case "workspace/symbol":
		var p WorkspaceSymbolParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.workspaceSymbols(p)
	case "textDocument/completion":
		var p CompletionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.completion(p)
	case "textDocument/formatting":
		var p FormattingParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.formatting(p)
	case "textDocument/documentSymbol":
		var p DocumentSymbolParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.documentSymbols(p)
	case "textDocument/signatureHelp":
		var p TextDocumentPositionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, errInvalidParams
		}
		return s.signatureHelp(p)
	}
	return nil, errMethodNotFound
}

func (s *server) handleNotify(method string, params json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch method {
	case "initialized":
		return nil
	case "textDocument/didOpen":
		var p DidOpenParams
		if err := json.Unmarshal(params, &p); err != nil {
			s.logf("didOpen: %v", err)
			return nil
		}
		path, err := uriToPath(p.TextDocument.URI)
		if err != nil {
			s.logf("didOpen: %v", err)
			return nil
		}
		s.docs[path] = &document{text: p.TextDocument.Text, version: p.TextDocument.Version}
		return s.reload(true)
	case "textDocument/didChange":
		var p DidChangeParams
		if err := json.Unmarshal(params, &p); err != nil {
			s.logf("didChange: %v", err)
			return nil
		}
		path, err := uriToPath(p.TextDocument.URI)
		if err != nil {
			s.logf("didChange: %v", err)
			return nil
		}
		doc := s.docs[path]
		if doc == nil {
			return nil
		}
		doc.text = applyChanges(doc.text, p.ContentChanges)
		doc.version = p.TextDocument.Version
		return s.reload(false)
	case "textDocument/didSave":
		var p DidSaveParams
		if err := json.Unmarshal(params, &p); err != nil {
			s.logf("didSave: %v", err)
			return nil
		}
		path, err := uriToPath(p.TextDocument.URI)
		if err != nil {
			s.logf("didSave: %v", err)
			return nil
		}
		if doc := s.docs[path]; doc != nil && p.Text != nil {
			doc.text = *p.Text
		}
		return s.reload(true)
	case "textDocument/didClose":
		var p DidCloseParams
		if err := json.Unmarshal(params, &p); err != nil {
			s.logf("didClose: %v", err)
			return nil
		}
		path, err := uriToPath(p.TextDocument.URI)
		if err != nil {
			s.logf("didClose: %v", err)
			return nil
		}
		uri := pathToURI(path)
		if err := s.conn.Notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{URI: uri, Diagnostics: []Diagnostic{}}); err != nil {
			return err
		}
		delete(s.docs, path)
		return s.reload(true)
	}
	return nil
}

func decodeParams(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return errInvalidParams
	}
	return json.Unmarshal(raw, v)
}

func (s *server) initialize(p InitializeParams) {
	dir := initializeDir(p)
	if dir == "" {
		dir = s.rootArg
	}
	root := findProjectRoot(dir)
	s.root = root
	s.module = ""
	if root == "" {
		return
	}
	if cfg, err := config.Load(root); err == nil {
		s.module = cfg.Module
		return
	} else {
		if mod, merr := config.ReadModule(root); merr == nil {
			s.module = mod
		}
		s.logf("config: %v", err)
	}
}

func initializeDir(p InitializeParams) string {
	if p.RootURI != "" {
		if path, err := uriToPath(p.RootURI); err == nil {
			return path
		}
	}
	if len(p.WorkspaceFolders) > 0 {
		if path, err := uriToPath(p.WorkspaceFolders[0].URI); err == nil {
			return path
		}
	}
	if p.RootPath != "" {
		return p.RootPath
	}
	return ""
}

func findProjectRoot(dir string) string {
	if dir == "" {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	for {
		if config.Exists(abs) {
			return abs
		}
		if _, err := os.Stat(filepath.Join(abs, "main.ink")); err == nil {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}

func (s *server) logf(format string, args ...any) {
	fmt.Fprintf(s.stderr, "ink lsp: %s\n", fmt.Sprintf(format, args...))
}

func pathToURI(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.ToSlash(filepath.Clean(path))
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func uriToPath(uri string) (string, error) {
	if uri == "" {
		return "", errors.New("empty uri")
	}
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme != "" && !strings.EqualFold(u.Scheme, "file") {
		return "", fmt.Errorf("unsupported URI scheme %q", u.Scheme)
	}
	path := u.Path
	if path == "" && u.Opaque != "" {
		path = u.Opaque
	}
	if u.Host != "" && u.Host != "localhost" {
		path = "//" + u.Host + path
	}
	if path == "" {
		return "", errors.New("empty URI path")
	}
	abs, err := filepath.Abs(filepath.FromSlash(path))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func applyChanges(text string, changes []TextDocumentChange) string {
	for _, change := range changes {
		if change.Range == nil {
			text = change.Text
			continue
		}
		src := []byte(text)
		start := OffsetAt(src, change.Range.Start)
		end := OffsetAt(src, change.Range.End)
		if start > end {
			start, end = end, start
		}
		text = string(src[:start]) + change.Text + string(src[end:])
	}
	return text
}

func (s *server) reload(typecheckOn bool) error {
	s.gen++
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	gen := s.gen
	docs := make(map[string]*document, len(s.docs))
	for path, doc := range s.docs {
		copied := *doc
		docs[path] = &copied
	}
	root, module := s.root, s.module
	s.worker.submit(gen, func() any {
		return s.analyze(docs, root, module, typecheckOn, gen)
	})
	return nil
}

func (s *server) stop() {
	s.worker.close()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gen++
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *server) stale(gen int) bool {
	if gen == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return gen != s.gen
}

func (s *server) publishAnalysis(gen int, result any) {
	s.mu.Lock()
	if s.gen != gen {
		s.mu.Unlock()
		return
	}
	a := result.(*analysis)
	s.last = a
	if a.asyncTypecheck {
		s.scheduleTypecheck(a, gen)
	}
	params, cleared := s.publishPlan(a)
	s.mu.Unlock()
	if err := s.writePublish(params, cleared); err != nil {
		s.logf("publish: %v", err)
	}
}

func (s *server) analyze(docs map[string]*document, root, module string, typecheckOn bool, gen int) *analysis {
	a := &analysis{
		files:    make(map[string]*fileInfo),
		nodeFile: make(map[ast.Node]string),
		diags:    make(map[string][]Diagnostic),
		root:     root,
		module:   module,
	}
	parsed := make(map[string][]*ast.File)
	parseErrs := make(map[string][]parser.Error)
	parsedPath := make(map[string]bool)
	var semaErrs []sema.Error

	if root != "" {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				s.logf("%v", walkErr)
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if path == root {
					return nil
				}
				if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "vendor" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".ink") {
				return nil
			}
			full := filepath.Clean(path)
			if s.stale(gen) {
				return filepath.SkipAll
			}
			src, err := s.source(docs, full)
			if err != nil {
				s.logf("%v", err)
				return nil
			}
			f, errs := parser.Parse(full, src)
			if f == nil {
				return nil
			}
			rel := relDir(root, filepath.Dir(full))
			parsed[rel] = append(parsed[rel], f)
			parseErrs[full] = errs
			parsedPath[full] = true
			indexNodes(a.nodeFile, f, full)
			return nil
		})
		if err != nil {
			s.logf("%v", err)
		}
	}

	for _, path := range s.docPaths(docs) {
		if parsedPath[path] {
			continue
		}
		doc := docs[path]
		if doc == nil {
			continue
		}
		src := []byte(doc.text)
		f, errs := parser.Parse(path, src)
		if f == nil {
			continue
		}
		indexNodes(a.nodeFile, f, path)
		if projectFile(root, path) {
			rel := relDir(root, filepath.Dir(path))
			parsed[rel] = append(parsed[rel], f)
			parseErrs[path] = errs
			parsedPath[path] = true
			continue
		}
		prog := s.check(map[string][]*ast.File{".": {f}}, filepath.Dir(path), "", gen)
		if prog != nil {
			semaErrs = append(semaErrs, prog.Errors...)
		}
		fi := &fileInfo{path: path, rel: ".", src: src, file: f, prog: prog}
		if doc := docs[path]; doc != nil {
			fi.version, fi.open = doc.version, true
		}
		a.files[path] = fi
		parseErrs[path] = errs
	}

	if s.stale(gen) {
		return a
	}
	if len(parsed) > 0 {
		prog := s.check(parsed, root, module, gen)
		if s.stale(gen) {
			return a
		}
		if prog != nil {
			semaErrs = append(semaErrs, prog.Errors...)
		}
		a.prog = prog
		a.parsed = parsed
		for rel, files := range parsed {
			for _, f := range files {
				fi := &fileInfo{path: f.Path, rel: rel, src: f.Src, file: f, prog: prog}
				if doc := docs[f.Path]; doc != nil {
					fi.version, fi.open = doc.version, true
				}
				a.files[f.Path] = fi
			}
		}
		if s.stale(gen) {
			return a
		}
		if prog != nil && canTypecheck(root, module) && !hasFatalErrors(parseErrs, prog.Errors) {
			if typecheckOn {
				s.addTypecheckDiags(a, prog, parsed)
			} else {
				a.asyncTypecheck = true
			}
		}
	}

	for path, errs := range parseErrs {
		fi := a.files[path]
		if fi == nil {
			continue
		}
		for _, e := range errs {
			a.diags[path] = append(a.diags[path], diagnosticAt(fi.src, e.Pos, e.Msg, false))
		}
	}
	for _, e := range semaErrs {
		fi := a.files[canonicalPath(e.Path)]
		if fi == nil && e.Path != "" && !filepath.IsAbs(e.Path) && root != "" {
			fi = a.files[canonicalPath(filepath.Join(root, filepath.FromSlash(e.Path)))]
		}
		if fi == nil {
			fi = a.fileAtPos(e.Pos)
		}
		if fi == nil {
			continue
		}
		a.diags[fi.path] = append(a.diags[fi.path], diagnosticAt(fi.src, e.Pos, e.Msg, e.Warning))
	}
	for _, diags := range a.diags {
		sortDiagnostics(diags)
	}

	return a
}

func (s *server) addTypecheckDiags(a *analysis, prog *sema.Program, parsed map[string][]*ast.File) {
	s.mergeDiags(a, s.typecheckDiagnostics(a, prog, parsed))
}

func (s *server) typecheckDiagnostics(a *analysis, prog *sema.Program, parsed map[string][]*ast.File) map[string][]Diagnostic {
	out := make(map[string][]Diagnostic)
	if prog == nil {
		return out
	}
	for _, e := range typecheck.Check(prog, parsed, typecheck.Options{Root: a.root, Module: a.module, Tests: true}) {
		fi := a.files[canonicalPath(e.Path)]
		if fi == nil {
			s.logf("typecheck: %s", e.Msg)
			continue
		}
		d := diagnosticAt(fi.src, e.Pos, e.Msg, false)
		d.Code = diag.Typecheck(e.Msg).Code
		out[fi.path] = append(out[fi.path], d)
	}
	return out
}

func (s *server) mergeDiags(a *analysis, diags map[string][]Diagnostic) {
	for path, list := range diags {
		a.diags[path] = append(a.diags[path], list...)
	}
	for _, list := range a.diags {
		sortDiagnostics(list)
	}
}

func (s *server) scheduleTypecheck(a *analysis, gen int) {
	s.timer = time.AfterFunc(typecheckDelay, func() {
		s.runTypecheck(a, gen)
	})
}

func (s *server) runTypecheck(a *analysis, gen int) {
	s.mu.Lock()
	if s.last != a || s.gen != gen {
		s.mu.Unlock()
		return
	}
	prog, parsed := a.prog, a.parsed
	s.mu.Unlock()

	diags := s.typecheckDiagnostics(a, prog, parsed)

	s.mu.Lock()
	if s.last != a || s.gen != gen {
		s.mu.Unlock()
		return
	}
	s.mergeDiags(a, diags)
	params, cleared := s.publishPlan(a)
	s.mu.Unlock()
	if err := s.writePublish(params, cleared); err != nil {
		s.logf("typecheck publish: %v", err)
	}
}

func canTypecheck(root, module string) bool {
	if root == "" || module == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(root, "go.mod"))
	return err == nil
}

func hasFatalErrors(parseErrs map[string][]parser.Error, errs []sema.Error) bool {
	for _, list := range parseErrs {
		if len(list) > 0 {
			return true
		}
	}
	for _, err := range errs {
		if !err.Warning {
			return true
		}
	}
	return false
}

func (s *server) check(parsed map[string][]*ast.File, root, module string, gen int) (prog *sema.Program) {
	defer func() {
		if r := recover(); r != nil {
			s.logf("analysis panic: %v", r)
			prog = nil
		}
	}()
	return sema.Check(parsed, sema.Options{Root: root, Module: module, GoCache: s.cache, Stop: func() bool { return s.stale(gen) }})
}

func (a *analysis) fileAtPos(pos token.Pos) *fileInfo {
	if !pos.Valid() {
		return nil
	}
	paths := make([]string, 0, len(a.files))
	for path := range a.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fi := a.files[path]
		found := false
		ast.Inspect(fi.file, func(n ast.Node) bool {
			if owner, ok := a.nodeFile[n]; ok && owner != fi.path {
				return false
			}
			p := n.Pos()
			if p.Offset == pos.Offset && p.Line == pos.Line && p.Col == pos.Col {
				found = true
				return false
			}
			return true
		})
		if found {
			return fi
		}
	}
	return nil
}

func (s *server) source(docs map[string]*document, path string) ([]byte, error) {
	if doc := docs[path]; doc != nil {
		return []byte(doc.text), nil
	}
	return os.ReadFile(path)
}

func (s *server) docPaths(docs map[string]*document) []string {
	paths := make([]string, 0, len(docs))
	for path := range docs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func indexNodes(index map[ast.Node]string, f *ast.File, path string) {
	ast.Inspect(f, func(n ast.Node) bool {
		if n != nil {
			index[n] = path
		}
		return true
	})
}

func relDir(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return filepath.ToSlash(dir)
	}
	rel = filepath.ToSlash(rel)
	if rel == "" {
		return "."
	}
	return rel
}

func projectFile(root, path string) bool {
	if root == "" || !strings.HasSuffix(path, ".ink") {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return false
	}
	parts := strings.Split(rel, "/")
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") || part == "vendor" || part == "node_modules" {
			return false
		}
	}
	return true
}

func diagnosticAt(src []byte, pos token.Pos, msg string, warning bool) Diagnostic {
	severity := 1
	if warning {
		severity = 2
	}
	info := diag.Classify(msg)
	if info.Hint != "" {
		msg += " (" + info.Hint + ")"
	}
	return Diagnostic{Range: rangeAt(src, pos), Severity: severity, Code: info.Code, Source: "ink", Message: msg}
}

func rangeAt(src []byte, pos token.Pos) Range {
	offset := pos.Offset
	bad := !pos.Valid() || offset < 0 || offset > len(src)
	if !bad && offset == 0 && (pos.Line > 1 || pos.Col > 1) {
		bad = true
	}
	if bad {
		offset = 0
		if pos.Valid() {
			line := 1
			start := 0
			for i := 0; i < len(src) && line < pos.Line; i++ {
				if src[i] == '\n' {
					line++
					start = i + 1
				}
			}
			offset = start + pos.Col - 1
			if offset < 0 {
				offset = 0
			}
			if offset > len(src) {
				offset = len(src)
			}
		}
	}
	start := PositionAt(src, offset)
	end := start
	end.Character++
	return Range{Start: start, End: end}
}

func sortDiagnostics(diags []Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.Range.Start.Line != b.Range.Start.Line {
			return a.Range.Start.Line < b.Range.Start.Line
		}
		if a.Range.Start.Character != b.Range.Start.Character {
			return a.Range.Start.Character < b.Range.Start.Character
		}
		return a.Message < b.Message
	})
}

func (s *server) publishPlan(a *analysis) ([]PublishDiagnosticsParams, []string) {
	paths := make([]string, 0, len(a.files))
	for path := range a.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	params := make([]PublishDiagnosticsParams, 0, len(paths))
	current := make(map[string]bool, len(paths))
	for _, path := range paths {
		uri := pathToURI(path)
		diags := a.diags[path]
		if diags == nil {
			diags = []Diagnostic{}
		}
		p := PublishDiagnosticsParams{URI: uri, Diagnostics: diags}
		if fi := a.files[path]; fi != nil && fi.open {
			version := fi.version
			p.Version = &version
		}
		params = append(params, p)
		current[uri] = true
	}
	cleared := make([]string, 0)
	for uri := range s.published {
		if !current[uri] {
			cleared = append(cleared, uri)
		}
	}
	sort.Strings(cleared)
	s.published = current
	return params, cleared
}

func (s *server) writePublish(params []PublishDiagnosticsParams, cleared []string) error {
	for _, p := range params {
		if err := s.conn.Notify("textDocument/publishDiagnostics", p); err != nil {
			return err
		}
	}
	for _, uri := range cleared {
		if err := s.conn.Notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{URI: uri, Diagnostics: []Diagnostic{}}); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) fileAt(uri string) (*fileInfo, bool) {
	path, err := uriToPath(uri)
	if err != nil {
		return nil, false
	}
	if s.last == nil {
		return nil, true
	}
	fi := s.last.files[path]
	if fi == nil {
		return nil, true
	}
	if doc := s.docs[path]; doc != nil && doc.version != fi.version {
		return nil, true
	}
	return fi, true
}

func (s *server) hover(p TextDocumentPositionParams) (any, error) {
	fi, ok := s.fileAt(p.TextDocument.URI)
	if !ok {
		return nil, errInvalidParams
	}
	if fi == nil {
		return nil, nil
	}
	node := s.nodeAt(fi, OffsetAt(fi.src, p.Position))
	if node == nil {
		return nil, nil
	}
	value := hoverText(node)
	if extra := s.inferredType(fi, node, p.Position.Line+1, p.Position.Character+1, OffsetAt(fi.src, p.Position)); extra != "" {
		if value == "" {
			value = extra
		} else {
			value += "\n\n" + extra
		}
	}
	if value == "" {
		return nil, nil
	}
	r := spanRange(fi.src, node)
	return &Hover{Contents: MarkupContent{Kind: "markdown", Value: value}, Range: &r}, nil
}

func (s *server) inferredType(fi *fileInfo, node ast.Node, line, col, offset int) string {
	if fi == nil || node == nil || s.last == nil || s.last.prog == nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(s.last.root, "go.mod")); err != nil {
		return ""
	}
	if s.info == nil || s.infoGen != s.gen {
		s.info = typecheck.InferTypes(s.last.prog, s.last.parsed, typecheck.Options{Root: s.last.root, Module: s.last.module, Tests: true})
		s.infoGen = s.gen
	}
	if s.info == nil {
		println("DEBUG info nil")
		return ""
	}
	t := s.info.At[fi.path+":"+strconv.Itoa(line)+":"+strconv.Itoa(col)]
	if t == "" {
		if word := identWordAt(fi.src, offset); word != "" {
			t = s.info.ByName[filepath.Dir(fi.path)+"\x00"+strconv.Itoa(line)+"\x00"+word]
		}
	}
	if t == "" {
		pos := node.Pos()
		t = s.info.At[fi.path+":"+strconv.Itoa(pos.Line)+":"+strconv.Itoa(pos.Col)]
	}
	if t == "" {
		return ""
	}
	return codeBlock("inferred: " + inkTypeText(t))
}

var goTypeWord = regexp.MustCompile(`\b(int32|int64|uint8|uint32|uint64|float64|float32|string)\b`)

func inkTypeText(s string) string {
	s = strings.ReplaceAll(s, "[]byte", "u8[]")
	s = strings.ReplaceAll(s, "[]rune", "rune[]")
	return goTypeWord.ReplaceAllStringFunc(s, func(word string) string {
		switch word {
		case "int32":
			return "i32"
		case "int64":
			return "i64"
		case "uint8":
			return "u8"
		case "uint32":
			return "u32"
		case "uint64":
			return "u64"
		case "float64":
			return "f64"
		case "float32":
			return "f32"
		case "string":
			return "str"
		}
		return word
	})
}

func identWordAt(src []byte, offset int) string {
	if offset < 0 || offset > len(src) {
		return ""
	}
	start := offset
	for start > 0 && isWordByte(src[start-1]) {
		start--
	}
	end := offset
	for end < len(src) && isWordByte(src[end]) {
		end++
	}
	return string(src[start:end])
}

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func (s *server) nodeAt(fi *fileInfo, offset int) ast.Node {
	node := ast.NodeAt(fi.file, offset)
	if node == nil {
		return nil
	}
	if owner, ok := s.last.nodeFile[node]; ok && owner != fi.path {
		return s.ownedNodeAt(fi, offset)
	}
	return node
}

func (s *server) ownedNodeAt(fi *fileInfo, offset int) ast.Node {
	var best ast.Node
	bestWidth := int(^uint(0) >> 1)
	s.inspectFile(fi, func(n ast.Node) bool {
		start, end := n.Pos().Offset, n.EndPos().Offset
		if start <= offset && offset <= end {
			if width := end - start; width <= bestWidth {
				best = n
				bestWidth = width
			}
		}
		return true
	})
	return best
}

func (s *server) inspectFile(fi *fileInfo, fn func(ast.Node) bool) {
	ast.Inspect(fi.file, func(n ast.Node) bool {
		if owner, ok := s.last.nodeFile[n]; ok && owner != fi.path {
			return false
		}
		return fn(n)
	})
}

func hoverText(node ast.Node) string {
	switch node := node.(type) {
	case *ast.Ident:
		return refHover(node.Name, node.Ref)
	case *ast.FuncDecl:
		return codeBlock(funcDeclText(node))
	case *ast.TypeDecl:
		return codeBlock(typeDeclText(node))
	case *ast.VarDecl:
		return codeBlock(varDeclText(node))
	case *ast.BasicLit:
		if t := literalTypeName(node.Kind); t != "" {
			return codeBlock(t + " literal")
		}
	}
	return ""
}

func literalTypeName(kind token.Kind) string {
	switch kind {
	case token.String, token.RawString:
		return "str"
	case token.Int:
		return "int"
	case token.Float:
		return "f64"
	case token.Rune:
		return "rune"
	case token.Duration:
		return "Duration"
	case token.True, token.False:
		return "bool"
	}
	return ""
}

func codeBlock(text string) string {
	return "```ink\n" + text + "\n```"
}

func refHover(name string, ref *ast.Ref) string {
	if ref == nil {
		return ""
	}
	var code, kind string
	switch ref.Kind {
	case ast.RefFunc:
		if ref.Func != nil {
			code = funcDeclText(ref.Func)
		} else if t := formatType(ref.TypeVal); t != "" {
			code = t
		} else {
			code = name
		}
		kind = "function"
	case ast.RefType:
		switch {
		case ref.Type != nil:
			code = typeDeclText(ref.Type)
		default:
			if named, ok := ref.TypeVal.(*ast.NamedType); ok && named.Decl != nil {
				code = typeDeclText(named.Decl)
			} else {
				code = name
			}
		}
		kind = "type"
	case ast.RefPackage:
		code = name
		kind = "package"
	case ast.RefBuiltin:
		code = name
		kind = "builtin"
	case ast.RefSelf:
		code = name
		if t := formatType(ref.TypeVal); t != "" {
			code += ": " + t
		}
		kind = "self"
	default:
		code = name
		if t := formatType(ref.TypeVal); t != "" {
			code += ": " + t
		}
		kind = refKindName(ref.Kind)
	}
	if code == "" {
		return ""
	}
	return codeBlock(code) + "\n\n" + kind
}

func refKindName(kind ast.RefKind) string {
	switch kind {
	case ast.RefLocal:
		return "local variable"
	case ast.RefParam:
		return "parameter"
	case ast.RefGlobal:
		return "variable"
	case ast.RefConst:
		return "constant"
	case ast.RefFunc:
		return "function"
	case ast.RefType:
		return "type"
	case ast.RefField:
		return "field"
	case ast.RefSelf:
		return "self"
	case ast.RefBuiltin:
		return "builtin"
	case ast.RefEnumVariant:
		return "enum variant"
	case ast.RefPackage:
		return "package"
	}
	return "identifier"
}

func funcDeclText(fn *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("fn ")
	b.WriteString(fn.Name)
	if len(fn.TypeParams) > 0 {
		b.WriteString("[")
		for i, tp := range fn.TypeParams {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tp.Name)
		}
		b.WriteString("]")
	}
	b.WriteString("(")
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(paramLabel(p))
	}
	b.WriteString(") ")
	b.WriteString(resultText(fn.Results, fn.Err))
	return b.String()
}

func paramLabel(p *ast.Param) string {
	t := formatType(p.Type)
	switch {
	case p.Name != "" && t != "":
		return p.Name + ": " + t
	case p.Name != "":
		return p.Name
	case t != "":
		return t
	}
	return "_"
}

func resultText(results []ast.Type, hasError bool) string {
	text := "void"
	if len(results) > 0 {
		parts := make([]string, 0, len(results))
		for _, r := range results {
			if t := formatType(r); t != "" {
				parts = append(parts, t)
			}
		}
		if len(parts) > 0 {
			text = strings.Join(parts, ", ")
		}
	}
	if hasError {
		if strings.Contains(text, ", ") {
			text = "(" + text + ")"
		}
		text += "!"
	}
	return text
}

func funcTypeResultText(results []ast.Type, hasError bool) string {
	if len(results) == 0 {
		if hasError {
			return " void!"
		}
		return ""
	}
	parts := make([]string, 0, len(results))
	for _, r := range results {
		parts = append(parts, formatType(r))
	}
	if len(parts) == 1 {
		if hasError {
			return " " + parts[0] + "!"
		}
		return " " + parts[0]
	}
	text := " (" + strings.Join(parts, ", ") + ")"
	if hasError {
		text += "!"
	}
	return text
}

func typeDeclText(d *ast.TypeDecl) string {
	switch d.Kind {
	case ast.TypeStruct:
		return "type " + d.Name + " = struct"
	case ast.TypeEnum:
		return "type " + d.Name + " = enum"
	case ast.TypeInterface:
		return "type " + d.Name + " = interface"
	}
	if target := formatType(d.Target); target != "" {
		return "type " + d.Name + " = " + target
	}
	return "type " + d.Name
}

func varDeclText(v *ast.VarDecl) string {
	keyword := "var"
	if v.Const {
		keyword = "const"
	}
	text := keyword + " " + strings.Join(v.Names, ", ")
	if t := formatType(v.Type); t != "" {
		text += ": " + t
	}
	return text
}

func formatType(t ast.Type) string {
	switch t := t.(type) {
	case nil:
		return ""
	case *ast.NamedType:
		if t.Package != "" {
			return t.Package + "." + t.Name
		}
		if t.Name != "" {
			return t.Name
		}
		return t.GoName
	case *ast.SliceType:
		return formatType(t.Elem) + "[]"
	case *ast.ArrayType:
		return formatType(t.Elem) + "[" + formatExpr(t.Len) + "]"
	case *ast.GenericType:
		parts := make([]string, 0, len(t.Args))
		for _, arg := range t.Args {
			parts = append(parts, formatType(arg))
		}
		return formatType(t.Base) + "[" + strings.Join(parts, ", ") + "]"
	case *ast.MapType:
		return "map[" + formatType(t.Key) + ", " + formatType(t.Value) + "]"
	case *ast.PointerType:
		return "*" + formatType(t.Elem)
	case *ast.FuncType:
		parts := make([]string, 0, len(t.Params))
		for _, param := range t.Params {
			parts = append(parts, formatType(param))
		}
		return "fn(" + strings.Join(parts, ", ") + ") " + funcTypeResultText(t.Results, t.Err)
	case *ast.InterfaceType:
		return "interface"
	case *ast.StructType:
		return "struct"
	case *ast.GoType:
		return t.Text
	}
	return ""
}

func formatExpr(e ast.Expr) string {
	switch e := e.(type) {
	case nil:
		return ""
	case *ast.BasicLit:
		return e.Value
	case *ast.Ident:
		return e.Name
	case *ast.ParenExpr:
		return "(" + formatExpr(e.X) + ")"
	case *ast.UnaryExpr:
		return e.Op.String() + formatExpr(e.X)
	case *ast.BinaryExpr:
		return formatExpr(e.X) + " " + e.Op.String() + " " + formatExpr(e.Y)
	}
	return ""
}

func spanRange(src []byte, node ast.Node) Range {
	start := node.Pos().Offset
	end := node.EndPos().Offset
	if start < 0 {
		start = 0
	}
	if start > len(src) {
		start = len(src)
	}
	if end < start {
		end = start
	}
	if end > len(src) {
		end = len(src)
	}
	return Range{Start: PositionAt(src, start), End: PositionAt(src, end)}
}

func (s *server) definition(p TextDocumentPositionParams) (any, error) {
	fi, ok := s.fileAt(p.TextDocument.URI)
	if !ok {
		return nil, errInvalidParams
	}
	if fi == nil {
		return nil, nil
	}
	node := s.nodeAt(fi, OffsetAt(fi.src, p.Position))
	ref := refOf(node)
	if ref == nil {
		return nil, nil
	}
	decl := directDecl(ref)
	if decl == nil && (ref.Go != nil || ref.Kind == ast.RefBuiltin || ref.Kind == ast.RefPackage) {
		return nil, nil
	}
	target := s.fileOfNode(decl)
	if target == nil && ref.Path != "" && decl != nil {
		target = s.last.files[canonicalPath(ref.Path)]
	}
	if target == nil {
		decl, target = s.findDecl(ref, fi)
	}
	if target == nil || decl == nil {
		return nil, nil
	}
	return Location{URI: pathToURI(target.path), Range: spanRange(target.src, decl)}, nil
}

func refOf(node ast.Node) *ast.Ref {
	switch node := node.(type) {
	case *ast.Ident:
		return node.Ref
	case *ast.SelectorExpr:
		return node.Ref
	}
	return nil
}

func directDecl(ref *ast.Ref) ast.Node {
	switch {
	case ref.Decl != nil:
		return ref.Decl
	case ref.Func != nil:
		return ref.Func
	case ref.Type != nil:
		return ref.Type
	case ref.Field != nil:
		return ref.Field
	case ref.Variant != nil:
		return ref.Variant
	case ref.Use != nil:
		return ref.Use
	}
	return nil
}

func (s *server) fileOfNode(node ast.Node) *fileInfo {
	if node == nil || s.last == nil {
		return nil
	}
	path, ok := s.last.nodeFile[node]
	if !ok {
		return nil
	}
	return s.last.files[path]
}

func (s *server) findDecl(ref *ast.Ref, cur *fileInfo) (ast.Node, *fileInfo) {
	paths := make([]string, 0, len(s.last.files))
	for path := range s.last.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for pass := 0; pass < 2; pass++ {
		for _, path := range paths {
			fi := s.last.files[path]
			same := fi.prog == cur.prog && fi.rel == cur.rel
			if pass == 0 && !same {
				continue
			}
			if node, ok := matchDecl(fi, ref); ok {
				return node, fi
			}
		}
	}
	return nil, nil
}

func matchDecl(fi *fileInfo, ref *ast.Ref) (ast.Node, bool) {
	if fi.file == nil || ref.Name == "" {
		return nil, false
	}
	switch ref.Kind {
	case ast.RefType:
		for _, decl := range fi.file.Decls {
			if d, ok := decl.(*ast.TypeDecl); ok && d.Name == ref.Name {
				return d, true
			}
		}
	case ast.RefFunc:
		for _, decl := range fi.file.Decls {
			if d, ok := decl.(*ast.FuncDecl); ok && d.Name == ref.Name {
				return d, true
			}
			if d, ok := decl.(*ast.TypeDecl); ok {
				for _, method := range d.Methods {
					if method.Name == ref.Name {
						return method, true
					}
				}
			}
		}
	case ast.RefField:
		for _, decl := range fi.file.Decls {
			if d, ok := decl.(*ast.TypeDecl); ok {
				for _, field := range d.Fields {
					if field.Name == ref.Name {
						return field, true
					}
				}
				for _, variant := range d.Variants {
					for _, field := range variant.Fields {
						if field.Name == ref.Name {
							return field, true
						}
					}
				}
			}
		}
	case ast.RefEnumVariant:
		for _, decl := range fi.file.Decls {
			if d, ok := decl.(*ast.TypeDecl); ok {
				for _, variant := range d.Variants {
					if variant.Name == ref.Name {
						return variant, true
					}
				}
			}
		}
	default:
		for _, decl := range fi.file.Decls {
			if d, ok := decl.(*ast.VarDecl); ok {
				for _, name := range d.Names {
					if name == ref.Name {
						return d, true
					}
				}
			}
		}
	}
	return nil, false
}

func (s *server) completion(p CompletionParams) (any, error) {
	fi, ok := s.fileAt(p.TextDocument.URI)
	if !ok {
		return nil, errInvalidParams
	}
	items := []CompletionItem{}
	if fi == nil {
		return CompletionList{Items: items}, nil
	}
	offset := OffsetAt(fi.src, p.Position)
	prefix := prefixAt(fi.src, offset)
	if dot := strings.LastIndex(prefix, "."); dot >= 0 {
		alias := prefix[:dot]
		if last := strings.LastIndex(alias, "."); last >= 0 {
			alias = alias[last+1:]
		}
		if alias == "" {
			return CompletionList{Items: items}, nil
		}
		if id := s.valueBeforeDot(fi, offset-len(alias)-1); id != nil {
			return CompletionList{Items: s.fieldItems(id)}, nil
		}
		return CompletionList{Items: s.memberItems(fi, alias)}, nil
	}
	return CompletionList{Items: s.allItems(fi, offset)}, nil
}

func prefixAt(src []byte, offset int) string {
	if offset < 0 {
		offset = 0
	}
	if offset > len(src) {
		offset = len(src)
	}
	start := offset
	for start > 0 {
		r, size := utf8.DecodeLastRune(src[:start])
		if r == '.' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			start -= size
			continue
		}
		break
	}
	return string(src[start:offset])
}

func (s *server) valueBeforeDot(fi *fileInfo, offset int) *ast.Ident {
	if offset < 0 {
		return nil
	}
	node := s.nodeAt(fi, offset)
	for node != nil {
		if id, ok := node.(*ast.Ident); ok {
			if id.Ref != nil && (id.Ref.Kind == ast.RefLocal || id.Ref.Kind == ast.RefParam || id.Ref.Kind == ast.RefSelf || id.Ref.Kind == ast.RefGlobal) {
				return id
			}
			return nil
		}
		return nil
	}
	return nil
}

func (s *server) fieldItems(id *ast.Ident) []CompletionItem {
	items := []CompletionItem{}
	typ := id.Ref.TypeVal
	if p, ok := typ.(*ast.PointerType); ok {
		typ = p.Elem
	}
	named, ok := typ.(*ast.NamedType)
	if !ok || named.Decl == nil {
		return items
	}
	for _, f := range named.Decl.Fields {
		items = append(items, CompletionItem{Label: f.Name, Kind: ciField, Detail: formatType(f.Type), SortText: "0" + strings.ToLower(f.Name)})
	}
	for _, m := range named.Decl.Methods {
		items = append(items, CompletionItem{Label: m.Name, Kind: ciFunction, Detail: funcDeclText(m), SortText: "1" + strings.ToLower(m.Name)})
	}
	return sortItems(items)
}

func (s *server) memberItems(fi *fileInfo, alias string) []CompletionItem {
	items := []CompletionItem{}
	if fi.prog == nil || fi.file == nil {
		return items
	}
	var use *ast.Use
	for _, u := range fi.file.Uses {
		if useAlias(u) == alias {
			use = u
			break
		}
	}
	if use == nil {
		return items
	}
	pkg := fi.prog.Packages[fi.rel]
	if pkg == nil {
		return items
	}
	seen := make(map[string]bool)
	add := func(name string, kind int) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		items = append(items, CompletionItem{Label: name, Kind: kind, SortText: "0" + strings.ToLower(name)})
	}
	if use.Local {
		target := fi.prog.Packages[use.LocalDir]
		if target == nil {
			return items
		}
		for name, decl := range target.Types {
			if exported(name) {
				add(name, typeCompletionKind(decl))
			}
		}
		for name := range target.Funcs {
			if exported(name) {
				add(name, ciFunction)
			}
		}
		for name, decl := range target.Vars {
			if exported(name) {
				add(name, varCompletionKind(decl))
			}
		}
		return sortItems(items)
	}
	imp := pkg.Imports[alias]
	if imp == nil || imp.Go == nil || imp.Go.Types == nil {
		return items
	}
	scope := imp.Go.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj == nil || !obj.Exported() {
			continue
		}
		add(name, objectCompletionKind(obj))
	}
	return sortItems(items)
}

func useAlias(u *ast.Use) string {
	if u == nil {
		return ""
	}
	if u.Alias != "" {
		return u.Alias
	}
	if u.PackageName != "" {
		return u.PackageName
	}
	path := strings.TrimSuffix(u.Path, "/")
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		path = path[i+1:]
	}
	return path
}

func (s *server) allItems(fi *fileInfo, offset int) []CompletionItem {
	items := []CompletionItem{}
	seen := make(map[string]bool)
	add := func(name string, kind, group int) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		items = append(items, CompletionItem{Label: name, Kind: kind, SortText: fmt.Sprintf("%d%s", group, strings.ToLower(name))})
	}
	for _, name := range s.visibleLocals(fi, offset) {
		add(name, ciVariable, 0)
	}
	if fi.prog != nil {
		if pkg := fi.prog.Packages[fi.rel]; pkg != nil {
			for name, decl := range pkg.Types {
				add(name, typeCompletionKind(decl), 1)
			}
			for name := range pkg.Funcs {
				add(name, ciFunction, 1)
			}
			for name, decl := range pkg.Vars {
				add(name, varCompletionKind(decl), 1)
			}
		}
	} else if fi.file != nil {
		for _, decl := range fi.file.Decls {
			switch decl := decl.(type) {
			case *ast.TypeDecl:
				add(decl.Name, typeCompletionKind(decl), 1)
			case *ast.FuncDecl:
				add(decl.Name, ciFunction, 1)
			case *ast.VarDecl:
				for _, name := range decl.Names {
					add(name, varCompletionKind(decl), 1)
				}
			}
		}
	}
	for _, name := range builtinTypes {
		add(name, ciClass, 2)
	}
	for _, name := range builtinFuncs {
		add(name, ciFunction, 2)
	}
	for _, name := range keywords {
		add(name, ciKeyword, 3)
	}
	return sortItems(items)
}

func typeCompletionKind(d *ast.TypeDecl) int {
	switch d.Kind {
	case ast.TypeStruct:
		return ciStruct
	case ast.TypeEnum:
		return ciEnum
	case ast.TypeInterface:
		return 8
	}
	return ciClass
}

func varCompletionKind(d *ast.VarDecl) int {
	if d.Const {
		return ciConstant
	}
	return ciVariable
}

func objectCompletionKind(obj types.Object) int {
	switch obj.(type) {
	case *types.TypeName:
		return ciClass
	case *types.Func:
		return ciFunction
	case *types.Var:
		return ciVariable
	case *types.Const:
		return ciConstant
	case *types.Builtin:
		return ciFunction
	case *types.PkgName:
		return ciModule
	}
	return ciVariable
}

func sortItems(items []CompletionItem) []CompletionItem {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortText != items[j].SortText {
			return items[i].SortText < items[j].SortText
		}
		return items[i].Label < items[j].Label
	})
	return items
}

func exported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

func (s *server) visibleLocals(fi *fileInfo, offset int) []string {
	var bodies []*ast.Block
	var params [][]*ast.Param
	var owners []*ast.TypeDecl
	s.inspectFile(fi, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if containsNode(n, offset) {
				bodies = append(bodies, n.Body)
				params = append(params, n.Params)
				owners = append(owners, n.Owner)
			}
		case *ast.FuncLit:
			if containsNode(n, offset) {
				bodies = append(bodies, n.Body)
				params = append(params, n.Params)
				owners = append(owners, nil)
			}
		case *ast.TestDecl:
			if containsNode(n, offset) {
				bodies = append(bodies, n.Body)
				params = append(params, nil)
				owners = append(owners, nil)
			}
		}
		return true
	})
	if len(bodies) == 0 {
		return nil
	}
	names := []string{}
	for i, body := range bodies {
		for _, p := range params[i] {
			addLocalName(p.Name, &names)
		}
		if owners[i] != nil {
			for _, field := range owners[i].Fields {
				addLocalName(field.Name, &names)
			}
		}
		collectBlockLocals(body, offset, &names)
	}
	return names
}

func containsNode(node ast.Node, offset int) bool {
	if node == nil {
		return false
	}
	return node.Pos().Offset <= offset && offset <= node.EndPos().Offset
}

func addLocalName(name string, names *[]string) {
	if name == "" || name == "_" {
		return
	}
	*names = append(*names, name)
}

func collectBlockLocals(block *ast.Block, offset int, names *[]string) {
	if block == nil || !containsNode(block, offset) {
		return
	}
	for _, stmt := range block.Stmts {
		if stmt == nil {
			continue
		}
		if stmt.EndPos().Offset <= offset {
			if decl, ok := stmt.(*ast.DeclStmt); ok && decl.Decl != nil {
				for _, name := range decl.Decl.Names {
					addLocalName(name, names)
				}
			}
			continue
		}
		if stmt.Pos().Offset > offset {
			break
		}
		collectStmtLocals(stmt, offset, names)
		break
	}
}

func collectStmtLocals(stmt ast.Stmt, offset int, names *[]string) {
	switch stmt := stmt.(type) {
	case *ast.Block:
		collectBlockLocals(stmt, offset, names)
	case *ast.DeclStmt:
		if stmt.Decl != nil {
			for _, value := range stmt.Decl.Values {
				collectExprLocals(value, offset, names)
			}
		}
	case *ast.AssignStmt:
		for _, expr := range stmt.Lhs {
			collectExprLocals(expr, offset, names)
		}
		for _, expr := range stmt.Rhs {
			collectExprLocals(expr, offset, names)
		}
	case *ast.ExprStmt:
		collectExprLocals(stmt.X, offset, names)
	case *ast.ReturnStmt:
		for _, expr := range stmt.Results {
			collectExprLocals(expr, offset, names)
		}
	case *ast.DeferStmt:
		if stmt.Call != nil {
			collectExprLocals(stmt.Call, offset, names)
		}
	case *ast.IfStmt:
		collectExprLocals(stmt.Cond, offset, names)
		if stmt.Then != nil && containsNode(stmt.Then, offset) {
			collectBlockLocals(stmt.Then, offset, names)
		}
		if stmt.Else != nil && containsNode(stmt.Else, offset) {
			collectStmtLocals(stmt.Else, offset, names)
		}
	case *ast.ForStmt:
		if stmt.Range != nil {
			for _, name := range stmt.Range.Names {
				addLocalName(name, names)
			}
			collectExprLocals(stmt.Range.X, offset, names)
		} else {
			if decl, ok := stmt.Init.(*ast.DeclStmt); ok && decl.Decl != nil {
				for _, name := range decl.Decl.Names {
					addLocalName(name, names)
				}
			}
			if stmt.Init != nil {
				collectStmtLocals(stmt.Init, offset, names)
			}
			collectExprLocals(stmt.Cond, offset, names)
			if stmt.Post != nil {
				collectStmtLocals(stmt.Post, offset, names)
			}
		}
		collectExprLocals(stmt.Limit, offset, names)
		if stmt.Body != nil && containsNode(stmt.Body, offset) {
			collectBlockLocals(stmt.Body, offset, names)
		}
	case *ast.ParStmt:
		if stmt.Body != nil && containsNode(stmt.Body, offset) {
			collectBlockLocals(stmt.Body, offset, names)
		}
	case *ast.WithinStmt:
		collectExprLocals(stmt.Duration, offset, names)
		if stmt.Body != nil && containsNode(stmt.Body, offset) {
			collectBlockLocals(stmt.Body, offset, names)
		}
	}
}

func collectExprLocals(expr ast.Expr, offset int, names *[]string) {
	if expr == nil {
		return
	}
	ast.Inspect(expr, func(n ast.Node) bool {
		if _, ok := n.(*ast.TypeDecl); ok {
			return false
		}
		switch n := n.(type) {
		case *ast.MatchExpr:
			if containsNode(n, offset) {
				for _, cs := range n.Cases {
					if cs == nil || !containsNode(cs, offset) {
						continue
					}
					if cs.Pat != nil {
						for _, name := range cs.Pat.Fields {
							addLocalName(name, names)
						}
					}
					if cs.Block != nil && containsNode(cs.Block, offset) {
						collectBlockLocals(cs.Block, offset, names)
					}
				}
			}
		case *ast.OrExpr:
			if containsNode(n, offset) && n.Block != nil && containsNode(n.Block, offset) {
				addLocalName(n.Name, names)
				collectBlockLocals(n.Block, offset, names)
			}
		}
		return true
	})
}

func (s *server) formatting(p FormattingParams) (any, error) {
	path, err := uriToPath(p.TextDocument.URI)
	if err != nil {
		return nil, errInvalidParams
	}
	src, err := s.source(s.docs, path)
	if err != nil {
		return []TextEdit{}, nil
	}
	f, errs := parser.Parse(path, src)
	if f == nil || len(errs) > 0 {
		return []TextEdit{}, nil
	}
	out := format.File(f)
	return []TextEdit{{
		Range:   Range{Start: Position{}, End: PositionAt(src, len(src))},
		NewText: string(out),
	}}, nil
}

func (s *server) documentSymbols(p DocumentSymbolParams) (any, error) {
	fi, ok := s.fileAt(p.TextDocument.URI)
	if !ok {
		return nil, errInvalidParams
	}
	symbols := []DocumentSymbol{}
	if fi == nil || fi.file == nil {
		return symbols, nil
	}
	for _, decl := range fi.file.Decls {
		switch decl := decl.(type) {
		case *ast.TypeDecl:
			symbol := DocumentSymbol{
				Name:           decl.Name,
				Kind:           typeSymbolKind(decl),
				Range:          spanRange(fi.src, decl),
				SelectionRange: nameRange(fi.src, decl, decl.Name),
			}
			if decl.Kind == ast.TypeEnum {
				for _, variant := range decl.Variants {
					symbol.Children = append(symbol.Children, DocumentSymbol{
						Name:           variant.Name,
						Kind:           dskEnumMember,
						Range:          spanRange(fi.src, variant),
						SelectionRange: nameRange(fi.src, variant, variant.Name),
					})
				}
			}
			if decl.Kind == ast.TypeStruct {
				for _, method := range decl.Methods {
					symbol.Children = append(symbol.Children, DocumentSymbol{
						Name:           method.Name,
						Kind:           dskMethod,
						Range:          spanRange(fi.src, method),
						SelectionRange: nameRange(fi.src, method, method.Name),
					})
				}
			}
			symbols = append(symbols, symbol)
		case *ast.FuncDecl:
			symbols = append(symbols, DocumentSymbol{
				Name:           decl.Name,
				Kind:           dskFunction,
				Range:          spanRange(fi.src, decl),
				SelectionRange: nameRange(fi.src, decl, decl.Name),
			})
		case *ast.VarDecl:
			for _, name := range decl.Names {
				kind := dskVariable
				if decl.Const {
					kind = dskConstant
				}
				symbols = append(symbols, DocumentSymbol{
					Name:           name,
					Kind:           kind,
					Range:          spanRange(fi.src, decl),
					SelectionRange: nameRange(fi.src, decl, name),
				})
			}
		case *ast.TestDecl:
			symbols = append(symbols, DocumentSymbol{
				Name:           decl.Name,
				Kind:           dskFunction,
				Range:          spanRange(fi.src, decl),
				SelectionRange: testNameRange(fi.src, decl),
			})
		}
	}
	return symbols, nil
}

func typeSymbolKind(d *ast.TypeDecl) int {
	switch d.Kind {
	case ast.TypeStruct:
		return dskStruct
	case ast.TypeEnum:
		return dskEnum
	case ast.TypeInterface:
		return dskInterface
	}
	return dskClass
}

func nameRange(src []byte, node ast.Node, name string) Range {
	start := node.Pos().Offset
	end := node.EndPos().Offset
	if start < 0 {
		start = 0
	}
	if end > len(src) {
		end = len(src)
	}
	if start > end {
		start = end
	}
	if name != "" {
		if a, b, ok := findWord(src, start, end, name); ok {
			return Range{Start: PositionAt(src, a), End: PositionAt(src, b)}
		}
	}
	finish := start + len(name)
	if finish > end {
		finish = end
	}
	if finish < start {
		finish = start
	}
	return Range{Start: PositionAt(src, start), End: PositionAt(src, finish)}
}

func testNameRange(src []byte, decl *ast.TestDecl) Range {
	start := decl.Pos().Offset
	end := decl.EndPos().Offset
	if start < 0 {
		start = 0
	}
	if end > len(src) {
		end = len(src)
	}
	for i := start; i < end; i++ {
		if src[i] != '"' && src[i] != '`' {
			continue
		}
		quote := src[i]
		for j := i + 1; j < end; j++ {
			if quote != '`' && src[j] == '\\' {
				j++
				continue
			}
			if src[j] == quote {
				return Range{Start: PositionAt(src, i), End: PositionAt(src, j+1)}
			}
		}
		break
	}
	return nameRange(src, decl, decl.Name)
}

func findWord(src []byte, start, end int, word string) (int, int, bool) {
	if start < 0 {
		start = 0
	}
	if end > len(src) {
		end = len(src)
	}
	if word == "" || start >= end {
		return 0, 0, false
	}
	needle := []byte(word)
	for i := start; i+len(needle) <= end; {
		rel := bytes.Index(src[i:end], needle)
		if rel < 0 {
			return 0, 0, false
		}
		a := i + rel
		b := a + len(needle)
		before := a == 0 || !identByte(src[a-1])
		after := b >= len(src) || !identByte(src[b])
		if before && after {
			return a, b, true
		}
		i = a + 1
	}
	return 0, 0, false
}

func identByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func (s *server) signatureHelp(p TextDocumentPositionParams) (any, error) {
	fi, ok := s.fileAt(p.TextDocument.URI)
	if !ok {
		return nil, errInvalidParams
	}
	if fi == nil {
		return nil, nil
	}
	offset := OffsetAt(fi.src, p.Position)
	call := s.callAt(fi, offset)
	if call == nil {
		return nil, nil
	}
	return s.signature(fi, call, offset), nil
}

func (s *server) callAt(fi *fileInfo, offset int) *ast.CallExpr {
	var best *ast.CallExpr
	bestWidth := int(^uint(0) >> 1)
	s.inspectFile(fi, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && containsNode(call, offset) {
			if width := call.EndPos().Offset - call.Pos().Offset; width <= bestWidth {
				best = call
				bestWidth = width
			}
		}
		return true
	})
	return best
}

func (s *server) signature(fi *fileInfo, call *ast.CallExpr, offset int) *SignatureHelp {
	if call.Slice {
		label := "[](size, cap)"
		if len(call.TypeArgs) == 1 {
			label = formatType(call.TypeArgs[0]) + label
		}
		active := 0
		if call.Size == nil {
			active = 1
		}
		return &SignatureHelp{
			Signatures: []SignatureInformation{{
				Label:      label,
				Parameters: []ParameterInformation{{Label: "size"}, {Label: "cap"}},
			}},
			ActiveParameter: active,
		}
	}
	var label string
	var params []ParameterInformation
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if fun.Ref != nil && fun.Ref.Func != nil {
			label, params = inkSignature(fun.Ref.Func)
		}
	case *ast.SelectorExpr:
		if fun.Ref != nil {
			if fun.Ref.Func != nil {
				label, params = inkSignature(fun.Ref.Func)
			} else if goFunc, ok := fun.Ref.Go.(*types.Func); ok {
				if sig, ok := goFunc.Type().(*types.Signature); ok {
					goLabel, goParams := goSignature(sig, fi)
					label = "fn " + goFunc.Name() + goLabel
					params = goParams
				}
			}
		}
	}
	if label == "" {
		if call.Info != nil {
			for _, param := range call.Info.ParamTypes {
				params = append(params, ParameterInformation{Label: formatType(param)})
			}
		}
		labels := make([]string, 0, len(params))
		for _, param := range params {
			labels = append(labels, param.Label)
		}
		label = "fn " + callName(call.Fun) + "(" + strings.Join(labels, ", ") + ")"
	}
	return &SignatureHelp{
		Signatures:      []SignatureInformation{{Label: label, Parameters: params}},
		ActiveParameter: activeParameter(fi.src, call, offset),
	}
}

func inkSignature(fn *ast.FuncDecl) (string, []ParameterInformation) {
	params := make([]ParameterInformation, 0, len(fn.Params))
	for _, param := range fn.Params {
		params = append(params, ParameterInformation{Label: paramLabel(param)})
	}
	return funcDeclText(fn), params
}

func callName(fun ast.Expr) string {
	switch fun := fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Name
	}
	return "fn"
}

func goSignature(sig *types.Signature, fi *fileInfo) (string, []ParameterInformation) {
	start := 0
	if sig.Params().Len() > 0 && isContextType(sig.Params().At(0).Type()) {
		start = 1
	}
	labels := make([]string, 0, sig.Params().Len()-start)
	params := make([]ParameterInformation, 0, sig.Params().Len()-start)
	for i := start; i < sig.Params().Len(); i++ {
		param := sig.Params().At(i)
		typ := param.Type()
		variadic := sig.Variadic() && i == sig.Params().Len()-1
		if variadic {
			if slice, ok := typ.(*types.Slice); ok {
				typ = slice.Elem()
			}
		}
		text := types.TypeString(typ, qualifier(fi))
		if param.Name() != "" {
			text = param.Name() + " " + text
		} else {
			text = "_ " + text
		}
		if variadic {
			text += "..."
		}
		labels = append(labels, text)
		params = append(params, ParameterInformation{Label: text})
	}
	results := sig.Results()
	resultTexts := make([]string, 0, results.Len())
	for i := 0; i < results.Len(); i++ {
		resultTexts = append(resultTexts, types.TypeString(results.At(i).Type(), qualifier(fi)))
	}
	label := "(" + strings.Join(labels, ", ") + ")"
	switch len(resultTexts) {
	case 1:
		label += " " + resultTexts[0]
	case 2:
		label += " (" + strings.Join(resultTexts, ", ") + ")"
	default:
		if len(resultTexts) > 2 {
			label += " (" + strings.Join(resultTexts, ", ") + ")"
		}
	}
	return label, params
}

func qualifier(fi *fileInfo) types.Qualifier {
	return func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		if fi.prog != nil {
			if current := fi.prog.Packages[fi.rel]; current != nil {
				for alias, imp := range current.Imports {
					if imp.Go != nil && imp.Go.Path == pkg.Path() {
						return alias
					}
				}
			}
		}
		return pkg.Name()
	}
}

func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

func activeParameter(src []byte, call *ast.CallExpr, offset int) int {
	start := call.Fun.EndPos().Offset
	end := call.EndPos().Offset
	if start < call.Pos().Offset {
		start = call.Pos().Offset
	}
	if end > len(src) {
		end = len(src)
	}
	if start < 0 {
		start = 0
	}
	if start > end {
		return 0
	}
	rel := bytes.IndexByte(src[start:end], '(')
	if rel < 0 {
		return 0
	}
	open := start + rel + 1
	if offset < open {
		return 0
	}
	if offset > end {
		offset = end
	}
	depth := 0
	count := 0
	var quote byte
	lineComment := false
	blockComment := false
	for i := open; i < offset; i++ {
		c := src[i]
		if lineComment {
			if c == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if c == '*' && i+1 < offset && src[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if quote != '`' && c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '/':
			if i+1 < offset && src[i+1] == '/' {
				lineComment = true
				i++
			} else if i+1 < offset && src[i+1] == '*' {
				blockComment = true
				i++
			}
		case '"', '\'', '`':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}
