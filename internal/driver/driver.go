package driver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/bongdo/ink/internal/agent"
	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/config"
	"github.com/bongdo/ink/internal/diag"
	"github.com/bongdo/ink/internal/format"
	"github.com/bongdo/ink/internal/gen"
	"github.com/bongdo/ink/internal/parser"
	"github.com/bongdo/ink/internal/sema"
	"github.com/bongdo/ink/internal/token"
	"github.com/bongdo/ink/internal/typecheck"
	inkrt "github.com/bongdo/ink/runtime"
)

const Version = "0.2.0"

const openCodeConfig = `{
  "$schema": "https://opencode.ai/config.json",
  "lsp": {
    "ink": {
      "command": ["ink", "lsp"],
      "extensions": [".ink"]
    }
  },
  "formatter": {
    "ink": {
      "command": ["ink", "fmt", "-w", "$FILE"],
      "extensions": [".ink"]
    }
  }
}
`

var ErrCheck = errors.New("check failed")

type Error struct {
	Path    string
	Pos     token.Pos
	End     token.Pos
	Msg     string
	Warning bool
	Code    string
	Hint    string
}

func (e Error) String() string {
	if e.Pos.Valid() {
		return fmt.Sprintf("%s:%d:%d: %s", e.Path, e.Pos.Line, e.Pos.Col, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Msg)
}

type JSONError struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	EndLine  int    `json:"endLine"`
	EndCol   int    `json:"endCol"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint"`
}

func sourceError(path string, pos, end token.Pos, msg string, warning bool) Error {
	info := diag.Classify(msg)
	return Error{Path: path, Pos: pos, End: spanEnd(pos, end), Msg: msg, Warning: warning, Code: info.Code, Hint: info.Hint}
}

func typecheckError(path string, pos token.Pos, msg string) Error {
	info := diag.Typecheck(msg)
	return Error{Path: path, Pos: pos, End: spanEnd(pos, token.Pos{}), Msg: msg, Code: info.Code, Hint: info.Hint}
}

func spanEnd(pos, end token.Pos) token.Pos {
	if end.Valid() && end.Offset > pos.Offset {
		return end
	}
	if !pos.Valid() {
		return token.Pos{Line: 1, Col: 2}
	}
	return token.Pos{Offset: pos.Offset + 1, Line: pos.Line, Col: pos.Col + 1}
}

type Project struct {
	Root    string
	Config  *config.Config
	Module  string
	Parsed  map[string][]*ast.File
	Program *sema.Program
	Errors  []Error
	Stub    string
}

type CheckOptions struct {
	Tidy      bool
	Typecheck bool
	Fast      bool
	Race      bool
}

func Open(dir string) (*Project, error) {
	root, err := config.FindRoot(dir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	p := &Project{Root: root, Config: cfg, Module: cfg.Module}
	if err := p.parse(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Project) parse() error {
	p.Parsed = make(map[string][]*ast.File)
	var errs []Error
	err := filepath.WalkDir(p.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path == p.Root {
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
		src, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, sourceError(path, token.Pos{}, token.Pos{}, err.Error(), false))
			return nil
		}
		f, perrs := parser.Parse(path, src)
		for _, e := range perrs {
			errs = append(errs, sourceError(path, e.Pos, e.End, e.Msg, false))
		}
		if f == nil {
			return nil
		}
		rel := p.relDir(filepath.Dir(path))
		p.Parsed[rel] = append(p.Parsed[rel], f)
		return nil
	})
	if err != nil {
		return err
	}
	for _, files := range p.Parsed {
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	}
	p.Errors = errs
	return nil
}

func (p *Project) relDir(dir string) string {
	rel, err := filepath.Rel(p.Root, dir)
	if err != nil {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(rel)
}

func (p *Project) GoImports() []string {
	seen := make(map[string]bool)
	var out []string
	for _, files := range p.Parsed {
		for _, f := range files {
			for _, u := range f.Uses {
				if u.Local || u.Path == "" || seen[u.Path] {
					continue
				}
				seen[u.Path] = true
				out = append(out, u.Path)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (p *Project) Check(opt CheckOptions) error {
	p.Errors = nil
	if err := p.parse(); err != nil {
		return err
	}
	if len(p.Parsed) == 0 {
		p.Errors = append(p.Errors, sourceError(p.Root, token.Pos{}, token.Pos{}, "no .ink files", false))
		return nil
	}
	if len(p.Errors) > 0 {
		return nil
	}
	if err := p.ensureModule(); err != nil {
		return err
	}
	imports := p.GoImports()
	if err := p.withImportStub(imports, opt.Tidy, func() error {
		p.Program = sema.Check(p.Parsed, sema.Options{Root: p.Root, Module: p.Module})
		for _, e := range p.Program.Errors {
			p.Errors = append(p.Errors, sourceError(e.Path, e.Pos, token.Pos{}, e.Msg, e.Warning))
		}
		return nil
	}); err != nil {
		return err
	}
	if opt.Typecheck && !opt.Fast && !p.HasErrors() {
		for _, e := range typecheck.Check(p.Program, p.Parsed, typecheck.Options{Root: p.Root, Module: p.Module, Tests: true, Race: opt.Race}) {
			p.Errors = append(p.Errors, typecheckError(e.Path, e.Pos, e.Msg))
		}
	}
	return nil
}

func (p *Project) ensureModule() error {
	gomod := filepath.Join(p.Root, "go.mod")
	if data, err := os.ReadFile(gomod); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				p.Module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
				return nil
			}
		}
		return errors.New("go.mod has no module line")
	} else if !os.IsNotExist(err) {
		return err
	}
	content := fmt.Sprintf("module %s\n\ngo %s\n", p.Config.Module, p.Config.Go)
	if err := os.WriteFile(gomod, []byte(content), 0o644); err != nil {
		return err
	}
	p.Module = p.Config.Module
	return nil
}

func (p *Project) withImportStub(imports []string, tidy bool, fn func() error) error {
	if len(imports) == 0 {
		return fn()
	}
	if !needsStub(imports) {
		return fn()
	}
	stub := filepath.Join(p.Root, "zz_ink_imports.go")
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n")
	for _, path := range imports {
		fmt.Fprintf(&b, "\t_ %q\n", path)
	}
	b.WriteString(")\n")
	if err := os.WriteFile(stub, []byte(b.String()), 0o644); err != nil {
		return err
	}
	p.Stub = stub
	defer func() {
		os.Remove(stub)
		p.Stub = ""
	}()
	if tidy && p.needsTidy(imports) {
		if out, err := p.goCommand("mod", "tidy"); err != nil {
			return fmt.Errorf("go mod tidy: %v\n%s", err, out)
		}
	}
	return fn()
}

func needsStub(imports []string) bool {
	for _, path := range imports {
		if strings.Contains(firstSegment(path), ".") {
			return true
		}
	}
	return false
}

func (p *Project) needsTidy(imports []string) bool {
	gomod, err := os.ReadFile(filepath.Join(p.Root, "go.mod"))
	if err != nil {
		return true
	}
	_, sumErr := os.Stat(filepath.Join(p.Root, "go.sum"))
	for _, path := range imports {
		if !strings.Contains(firstSegment(path), ".") {
			continue
		}
		prefix := modulePrefix(path)
		if !strings.Contains(string(gomod), prefix) {
			return true
		}
		if sumErr != nil {
			return true
		}
	}
	return false
}

func firstSegment(path string) string {
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return path
}

func modulePrefix(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, "/")
}

func (p *Project) env() []string {
	return config.GoEnv()
}

func (p *Project) goCommand(args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = p.Root
	cmd.Env = p.env()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (p *Project) Generate(tests bool) error {
	if p.Program == nil {
		return errors.New("project is not checked")
	}
	files, errs := gen.Generate(p.Program, p.Parsed, gen.Options{Root: p.Root, Module: p.Module, IncludeTests: tests})
	if len(errs) > 0 {
		for _, e := range errs {
			p.Errors = append(p.Errors, sourceError(p.Root, token.Pos{}, token.Pos{}, e.Error(), false))
		}
		return errs[0]
	}
	current := make(map[string]bool, len(files))
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return err
		}
		if _, err := writeIfChanged(f.Path, f.Src); err != nil {
			return err
		}
		current[p.relDir(f.Path)] = true
	}
	if tests {
		for _, pkg := range p.Program.Packages {
			if !pkgHasTests(pkg) {
				_ = os.Remove(filepath.Join(pkg.Dir, "zz_ink_test.go"))
			}
		}
	}
	for _, rel := range p.readManifest() {
		if current[rel] {
			continue
		}
		if !tests && strings.HasSuffix(rel, "zz_ink_test.go") {
			current[rel] = true
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			continue
		}
		if err := os.Remove(filepath.Join(p.Root, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := p.writeManifest(current); err != nil {
		return err
	}
	return p.writeRuntime()
}

func (p *Project) manifestPath() string {
	return filepath.Join(p.Root, ".ink", "generated.json")
}

func (p *Project) readManifest() []string {
	data, err := os.ReadFile(p.manifestPath())
	if err != nil {
		return nil
	}
	var files []string
	if json.Unmarshal(data, &files) != nil {
		return nil
	}
	return files
}

func (p *Project) writeManifest(current map[string]bool) error {
	files := make([]string, 0, len(current))
	for rel := range current {
		files = append(files, rel)
	}
	sort.Strings(files)
	data, err := json.Marshal(files)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.manifestPath()), 0o755); err != nil {
		return err
	}
	_, err = writeIfChanged(p.manifestPath(), append(data, '\n'))
	return err
}

func writeIfChanged(path string, data []byte) (bool, error) {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return false, nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func pkgHasTests(pkg *sema.Package) bool {
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			if _, ok := d.(*ast.TestDecl); ok {
				return true
			}
		}
	}
	return false
}

func (p *Project) writeRuntime() error {
	dir := filepath.Join(p.Root, ".ink", "runtime")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	_, err := writeIfChanged(filepath.Join(dir, "runtime.go"), inkrt.Source)
	return err
}

func (p *Project) HasErrors() bool {
	for _, e := range p.Errors {
		if !e.Warning {
			return true
		}
	}
	return false
}

func (p *Project) Build(out string, goArgs []string) error {
	if err := p.Check(CheckOptions{Tidy: true}); err != nil {
		return err
	}
	if p.HasErrors() {
		return ErrCheck
	}
	if err := p.Generate(false); err != nil {
		return err
	}
	if out == "" {
		out = filepath.Join("bin", p.Config.Name)
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(p.Root, out)
	}
	if info, err := os.Stat(out); err == nil && info.IsDir() {
		out = filepath.Join(out, p.Config.Name)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	args := append([]string{"build", "-buildvcs=false"}, goArgs...)
	args = append(args, "-o", out, ".")
	cmd := exec.Command("go", args...)
	cmd.Dir = p.Root
	cmd.Env = p.env()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed")
	}
	return nil
}

func (p *Project) Run(race bool, args []string) (int, error) {
	out := filepath.Join(p.Root, ".ink", "bin", p.Config.Name)
	var goArgs []string
	if race {
		goArgs = append(goArgs, "-race")
	}
	if err := p.Build(out, goArgs); err != nil {
		return 1, err
	}
	if p.HasErrors() {
		return 1, ErrCheck
	}
	return execRun(out, p.Root, args)
}

func execRun(bin, dir string, args []string) (int, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 1, err
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range sigs {
			_ = cmd.Process.Signal(sig)
		}
	}()
	err := cmd.Wait()
	signal.Stop(sigs)
	close(sigs)
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 1, err
}

func (p *Project) Test(args []string) (int, error) {
	if err := p.Check(CheckOptions{Tidy: true}); err != nil {
		return 1, err
	}
	if p.HasErrors() {
		return 1, ErrCheck
	}
	if err := p.Generate(true); err != nil {
		return 1, err
	}
	if !p.hasTests() {
		fmt.Fprintln(os.Stderr, "ink: no `test` blocks in .ink sources; declare `test \"name\" { ... };` in any .ink file")
	}
	cmd := exec.Command("go", append([]string{"test", "-buildvcs=false", "./..."}, args...)...)
	cmd.Dir = p.Root
	cmd.Env = p.env()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 1, err
}

func (p *Project) hasTests() bool {
	if p.Program == nil {
		return false
	}
	for _, pkg := range p.Program.Packages {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				if _, ok := d.(*ast.TestDecl); ok {
					return true
				}
			}
		}
	}
	return false
}

func Format(write, list, jsonOut bool, paths []string, stdout io.Writer) error {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if jsonOut {
		list = true
	}
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if p != path && (strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_") || d.Name() == "vendor") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), ".ink") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	sort.Strings(files)
	var listed []string
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, errs := parser.Parse(path, src)
		if len(errs) > 0 {
			return fmt.Errorf("%s:%d:%d: %s", path, errs[0].Pos.Line, errs[0].Pos.Col, errs[0].Msg)
		}
		out := format.File(f)
		differs := !bytes.Equal(out, src)
		if differs && list && stdout != nil {
			if jsonOut {
				listed = append(listed, path)
			} else if _, err := fmt.Fprintln(stdout, path); err != nil {
				return err
			}
		}
		if write {
			if err := os.WriteFile(path, out, 0o644); err != nil {
				return err
			}
			continue
		}
		if list {
			continue
		}
		if stdout != nil {
			if _, err := stdout.Write(out); err != nil {
				return err
			}
		}
	}
	if jsonOut && stdout != nil {
		if listed == nil {
			listed = []string{}
		}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(listed); err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) Diagnostics() []JSONError {
	out := make([]JSONError, 0, len(p.Errors))
	for _, e := range p.Errors {
		line, col := e.Pos.Line, e.Pos.Col
		if line < 1 {
			line = 1
		}
		if col < 1 {
			col = 1
		}
		endLine, endCol := e.End.Line, e.End.Col
		if endLine < 1 {
			endLine = line
		}
		if endCol < 1 {
			endCol = col + 1
		}
		severity := "error"
		if e.Warning {
			severity = "warning"
		}
		code, hint := e.Code, e.Hint
		if code == "" {
			info := diag.Classify(e.Msg)
			code, hint = info.Code, info.Hint
		}
		out = append(out, JSONError{
			File:     absPath(e.Path),
			Line:     line,
			Col:      col,
			EndLine:  endLine,
			EndCol:   endCol,
			Severity: severity,
			Code:     code,
			Message:  e.Msg,
			Hint:     hint,
		})
	}
	return out
}

func (p *Project) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(p.Diagnostics())
}

func absPath(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func (p *Project) PrintErrors(w io.Writer) {
	for _, e := range p.Errors {
		fmt.Fprintln(w, e.String())
	}
}

func New(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if entries, err := os.ReadDir(abs); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s is not empty", dir)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	name := filepath.Base(abs)
	cfg := &config.Config{Name: name, Module: "example.com/" + name, Go: "1.27"}
	if err := os.WriteFile(filepath.Join(abs, "AGENTS.md"), []byte(agent.ShortGuide), 0o644); err != nil {
		return err
	}
	main := fmt.Sprintf("use fmt;\n\nfn main() void! {\n    fmt.Println(%q) or fail;\n};\n", "hello from "+name)
	if err := os.WriteFile(filepath.Join(abs, "main.ink"), []byte(main), 0o644); err != nil {
		return err
	}
	gitignore := ".ink/\nbin/\nzz_ink.go\nzz_ink_test.go\n"
	if err := os.WriteFile(filepath.Join(abs, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(abs, "opencode.json"), []byte(openCodeConfig), 0o644); err != nil {
		return err
	}
	skillDir := filepath.Join(abs, ".opencode", "skills", "ink")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(agent.Skill), 0o644); err != nil {
		return err
	}
	gomod := fmt.Sprintf("module %s\n\ngo %s\n", cfg.Module, cfg.Go)
	return os.WriteFile(filepath.Join(abs, "go.mod"), []byte(gomod), 0o644)
}

func Init(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	if !config.Exists(abs) {
		name := filepath.Base(abs)
		cfg := &config.Config{Name: name, Module: "example.com/" + name, Go: "1.27"}
		if err := config.Save(abs, cfg); err != nil {
			return err
		}
	}
	agents := filepath.Join(abs, "AGENTS.md")
	if _, err := os.Stat(agents); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(agents, []byte(agent.ShortGuide), 0o644); err != nil {
			return err
		}
	}
	main := filepath.Join(abs, "main.ink")
	if _, err := os.Stat(main); os.IsNotExist(err) {
		src := "use fmt;\n\nfn main() void! {\n    fmt.Println(\"hello\") or fail;\n};\n"
		return os.WriteFile(main, []byte(src), 0o644)
	}
	return nil
}
