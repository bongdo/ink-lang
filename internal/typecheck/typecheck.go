package typecheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go/types"

	"golang.org/x/tools/go/packages"

	gotoken "go/token"

	"github.com/bongdo/ink-lang/internal/ast"
	"github.com/bongdo/ink-lang/internal/config"
	"github.com/bongdo/ink-lang/internal/gen"
	"github.com/bongdo/ink-lang/internal/sema"
	"github.com/bongdo/ink-lang/internal/token"
	inkrt "github.com/bongdo/ink-lang/runtime"
)

type Options struct {
	Root   string
	Module string
	Tests  bool
	Race   bool
	Vet    bool
}

type Error struct {
	Path string
	Pos  token.Pos
	Msg  string
	Vet  bool
}

var (
	posRE    = regexp.MustCompile(`^(.+?\.ink):(\d+)(?::(\d+))?$`)
	msgPosRE = regexp.MustCompile(`^(.+?\.ink):(\d+)(?::(\d+))?: (.+)$`)
)

// shortcut: local replace targets are not hashed, restart LSP after editing them
const cacheLimit = 16

var (
	cacheMu    sync.Mutex
	cache      = make(map[string][]Error)
	cacheOrder []string
)

func typecheckEnv() []string {
	return config.GoEnv()
}

func Check(p *sema.Program, files map[string][]*ast.File, opt Options) (out []Error) {
	defer func() {
		if r := recover(); r != nil {
			out = []Error{{Path: opt.Root, Msg: fmt.Sprintf("typecheck panic: %v", r)}}
		}
	}()
	module := moduleOf(opt)
	generated, errs := gen.Generate(p, files, gen.Options{Root: opt.Root, Module: module, IncludeTests: opt.Tests})
	if len(errs) > 0 {
		for _, err := range errs {
			out = append(out, Error{Path: opt.Root, Msg: err.Error()})
		}
		return out
	}
	if len(generated) == 0 {
		return nil
	}
	gomod, err := os.ReadFile(filepath.Join(opt.Root, "go.mod"))
	if err != nil {
		return []Error{{Path: opt.Root, Msg: "typecheck: " + err.Error()}}
	}
	gosum, _ := os.ReadFile(filepath.Join(opt.Root, "go.sum"))
	key := cacheKey(module, generated, gomod, gosum, opt.Tests, opt.Race)
	if errs, ok := cacheGet(key); ok {
		return errs
	}
	out = load(opt.Root, module, generated, knownFiles(files), opt.Tests, opt.Race)
	if len(out) == 0 && opt.Vet {
		out = vet(opt.Root, module, generated)
	}
	cachePut(opt.Root, key, out)
	return out
}

func importPath(root, module, rel string) string {
	if rel == "." {
		return module
	}
	return module + "/" + filepath.ToSlash(rel)
}

func patternsOf(root, module string, files []gen.File) []string {
	seen := make(map[string]bool)
	var patterns []string
	for _, f := range files {
		path, err := filepath.Abs(f.Path)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		pattern := "."
		if rel != "." {
			pattern = "./" + filepath.ToSlash(rel)
		}
		if !seen[pattern] {
			seen[pattern] = true
			patterns = append(patterns, pattern)
		}
	}
	sort.Strings(patterns)
	return patterns
}

func vet(root, module string, files []gen.File) []Error {
	dir, err := os.MkdirTemp("", "ink-vet")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)
	replace := make(map[string]string, len(files)+1)
	for i, f := range files {
		path, err := filepath.Abs(f.Path)
		if err != nil {
			continue
		}
		backing := filepath.Join(dir, "f"+strconv.Itoa(i)+".go")
		if err := os.WriteFile(backing, absoluteLineDirectives(f.Src, filepath.Dir(path)), 0o644); err != nil {
			return nil
		}
		replace[filepath.Clean(path)] = backing
	}
	runtimePath := filepath.Join(root, ".ink", "runtime", "runtime.go")
	runtimeBacking := filepath.Join(dir, "runtime.go")
	if err := os.WriteFile(runtimeBacking, inkrt.Source, 0o644); err != nil {
		return nil
	}
	replace[filepath.Clean(runtimePath)] = runtimeBacking
	overlay, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		return nil
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		return nil
	}
	cmd := exec.Command("go", append([]string{"vet", "-overlay=" + overlayPath}, patternsOf(root, module, files)...)...)
	cmd.Dir = root
	cmd.Env = typecheckEnv()
	out, _ := cmd.CombinedOutput()
	return vetErrors(root, string(out))
}

var vetLineRE = regexp.MustCompile(`^(.+\.ink):(\d+)(?::(\d+))?: (.+)$`)

func vetErrors(root, out string) []Error {
	var errs []Error
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		m := vetLineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNo, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		col := 1
		if m[3] != "" {
			if n, err := strconv.Atoi(m[3]); err == nil {
				col = n
			}
		}
		path := m[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, filepath.FromSlash(path))
		}
		if !underRoot(root, path) {
			continue
		}
		errs = append(errs, Error{Path: filepath.Clean(path), Pos: token.Pos{Line: lineNo, Col: col}, Msg: m[4], Vet: true})
	}
	return errs
}

var lineDirectiveRE = regexp.MustCompile(`//line (\S+):`)

func absoluteLineDirectives(src []byte, dir string) []byte {
	return []byte(lineDirectiveRE.ReplaceAllStringFunc(string(src), func(m string) string {
		path := strings.TrimSuffix(strings.TrimPrefix(m, "//line "), ":")
		return "//line " + filepath.Join(dir, path) + ":"
	}))
}

func knownFiles(files map[string][]*ast.File) map[string]bool {
	known := make(map[string]bool)
	for _, list := range files {
		for _, f := range list {
			if f == nil || f.Path == "" {
				continue
			}
			if abs, err := filepath.Abs(f.Path); err == nil {
				known[filepath.Clean(abs)] = true
			}
		}
	}
	return known
}

func cacheKey(module string, files []gen.File, gomod, gosum []byte, tests, race bool) string {
	h := sha256.New()
	io.WriteString(h, "ink-typecheck-v1\x00")
	if tests {
		io.WriteString(h, "tests\x00")
	}
	if race {
		io.WriteString(h, "race\x00")
	}
	io.WriteString(h, module)
	io.WriteString(h, "\x00")
	for _, f := range files {
		io.WriteString(h, f.Path)
		io.WriteString(h, "\x00")
		h.Write(f.Src)
		io.WriteString(h, "\x00")
	}
	h.Write(inkrt.Source)
	io.WriteString(h, "\x00")
	h.Write(gomod)
	io.WriteString(h, "\x00")
	h.Write(gosum)
	return hex.EncodeToString(h.Sum(nil))
}

func moduleOf(opt Options) string {
	if mod, err := config.ReadModule(opt.Root); err == nil {
		return mod
	}
	return opt.Module
}

func cacheGet(key string) ([]Error, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	errs, ok := cache[key]
	if !ok {
		return nil, false
	}
	return append([]Error(nil), errs...), true
}

func cachePut(root, key string, errs []Error) {
	for _, e := range errs {
		if !underRoot(root, e.Path) {
			return
		}
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if _, ok := cache[key]; ok {
		return
	}
	if len(cacheOrder) >= cacheLimit {
		delete(cache, cacheOrder[0])
		cacheOrder = cacheOrder[1:]
	}
	cache[key] = append([]Error(nil), errs...)
	cacheOrder = append(cacheOrder, key)
}

func load(root, module string, files []gen.File, known map[string]bool, tests, race bool) []Error {
	overlay := make(map[string][]byte, len(files)+1)
	importDirs := make(map[string]string, len(files))
	for _, f := range files {
		path, err := filepath.Abs(f.Path)
		if err != nil {
			return []Error{{Path: root, Msg: "typecheck: " + err.Error()}}
		}
		path = filepath.Clean(path)
		overlay[path] = f.Src
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		importDirs[importPath(root, module, rel)] = filepath.Dir(path)
	}
	patterns := patternsOf(root, module, files)
	if len(patterns) == 0 {
		return []Error{{Path: root, Msg: "typecheck: no generated packages"}}
	}
	overlay[filepath.Join(root, ".ink", "runtime", "runtime.go")] = inkrt.Source
	cfg := &packages.Config{
		Mode:    packages.LoadImports | packages.NeedExportFile,
		Dir:     root,
		Tests:   tests,
		Overlay: overlay,
		Env:     typecheckEnv(),
	}
	if race {
		cfg.BuildFlags = []string{"-race"}
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return []Error{{Path: root, Msg: "typecheck: " + err.Error()}}
	}
	if len(pkgs) == 0 {
		return []Error{{Path: root, Msg: "typecheck: no packages loaded"}}
	}

	var out []Error
	seenErr := make(map[string]bool)
	add := func(path string, line, col int, msg string) {
		key := path + "\x00" + strconv.Itoa(line) + "\x00" + strconv.Itoa(col) + "\x00" + msg
		if seenErr[key] {
			return
		}
		seenErr[key] = true
		out = append(out, Error{Path: path, Pos: token.Pos{Line: line, Col: col}, Msg: msg})
	}
	var other *packages.Error
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for i := range pkg.Errors {
			e := &pkg.Errors[i]
			if path, pos, ok := mapPos(root, known, e.Pos); ok {
				add(path, pos.Line, pos.Col, e.Msg)
				continue
			}
			if parseBundle(root, known, importDirs, e.Msg, add) {
				continue
			}
			if other == nil {
				other = e
			}
		}
	})
	if len(out) > 0 {
		return out
	}
	if other != nil {
		return []Error{{Path: root, Msg: "typecheck: " + other.Error()}}
	}
	return nil
}

func parseBundle(root string, known map[string]bool, importDirs map[string]string, msg string, add func(string, int, int, string)) bool {
	dir := root
	found := false
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			key := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			if i := strings.Index(key, " ["); i >= 0 {
				key = key[:i]
			}
			if d, ok := importDirs[key]; ok {
				dir = d
			}
			continue
		}
		m := msgPosRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNo, err := strconv.Atoi(m[2])
		if err != nil || lineNo < 1 {
			continue
		}
		col := 1
		if m[3] != "" {
			if n, err := strconv.Atoi(m[3]); err == nil && n > 0 {
				col = n
			}
		}
		path, ok := resolvePath(root, known, dir, m[1])
		if !ok {
			path, ok = resolveBase(root, known, dir, m[1])
		}
		if !ok {
			continue
		}
		add(path, lineNo, col, m[4])
		found = true
	}
	return found
}

func mapPos(root string, known map[string]bool, s string) (string, token.Pos, bool) {
	m := posRE.FindStringSubmatch(s)
	if m == nil {
		return "", token.Pos{}, false
	}
	line, err := strconv.Atoi(m[2])
	if err != nil || line < 1 {
		return "", token.Pos{}, false
	}
	col := 1
	if m[3] != "" {
		if n, err := strconv.Atoi(m[3]); err == nil && n > 0 {
			col = n
		}
	}
	path, ok := resolvePath(root, known, root, m[1])
	if !ok {
		return "", token.Pos{}, false
	}
	return path, token.Pos{Line: line, Col: col}, true
}

func resolvePath(root string, known map[string]bool, base, path string) (string, bool) {
	if filepath.IsAbs(path) {
		path = filepath.Clean(path)
		if underRoot(root, path) {
			return path, true
		}
		return "", false
	}
	path = filepath.FromSlash(path)
	for _, dir := range []string{root, base} {
		candidate := filepath.Clean(filepath.Join(dir, path))
		if !underRoot(root, candidate) {
			continue
		}
		if known[candidate] {
			return candidate, true
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func resolveBase(root string, known map[string]bool, dir, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	candidate := filepath.Clean(filepath.Join(dir, filepath.Base(filepath.FromSlash(path))))
	if !underRoot(root, candidate) {
		return "", false
	}
	if known[candidate] {
		return candidate, true
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate, true
	}
	return "", false
}

func underRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type Inferred struct {
	ByName map[string]string
	At     map[string]string
}

func InferTypes(p *sema.Program, files map[string][]*ast.File, opt Options) *Inferred {
	module := moduleOf(opt)
	generated, errs := gen.Generate(p, files, gen.Options{Root: opt.Root, Module: module, IncludeTests: true})
	if len(errs) > 0 || len(generated) == 0 {
		return nil
	}
	overlay := make(map[string][]byte, len(generated)+1)
	for _, f := range generated {
		path, err := filepath.Abs(f.Path)
		if err != nil {
			return nil
		}
		overlay[filepath.Clean(path)] = f.Src
	}
	overlay[filepath.Join(opt.Root, ".ink", "runtime", "runtime.go")] = inkrt.Source
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:     opt.Root,
		Overlay: overlay,
		Env:     typecheckEnv(),
	}
	pkgs, err := packages.Load(cfg, patternsOf(opt.Root, module, generated)...)
	if err != nil {
		return nil
	}
	out := &Inferred{ByName: make(map[string]string), At: make(map[string]string)}
	dirs := make(map[string]bool)
	for _, list := range files {
		for _, f := range list {
			dirs[filepath.Dir(f.Path)] = true
		}
	}
	qual := func(p *types.Package) string {
		if p == nil {
			return ""
		}
		return p.Name()
	}
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		for id, inst := range pkg.TypesInfo.Instances {
			pos := pkg.Fset.Position(id.Pos())
			text := types.TypeString(inst.Type, qual)
			if strings.Contains(text, "invalid type") {
				continue
			}
			out.At[resolvePosKey(opt.Root, dirs, pos)] = text
			out.ByName[resolveNameKey(opt.Root, dirs, pos, id.Name)] = text
		}
		for id, obj := range pkg.TypesInfo.Uses {
			pos := pkg.Fset.Position(id.Pos())
			key := resolvePosKey(opt.Root, dirs, pos)
			if _, ok := out.At[key]; ok {
				continue
			}
			text := types.TypeString(obj.Type(), qual)
			if strings.Contains(text, "invalid type") {
				continue
			}
			out.At[key] = text
			out.ByName[resolveNameKey(opt.Root, dirs, pos, obj.Name())] = text
		}
	}
	return out
}

func resolveNameKey(root string, dirs map[string]bool, pos gotoken.Position, name string) string {
	if name == "" {
		return ""
	}
	return filepath.Dir(resolveName(root, dirs, pos)) + "\x00" + strconv.Itoa(pos.Line) + "\x00" + name
}

func resolveName(root string, dirs map[string]bool, pos gotoken.Position) string {
	name := pos.Filename
	if name == "" {
		return ""
	}
	if !filepath.IsAbs(name) {
		for _, dir := range append([]string{root}, mapKeys(dirs)...) {
			if cand := filepath.Join(dir, name); fileExists(cand) {
				name = cand
				break
			}
		}
	}
	return filepath.Clean(name)
}

func mapKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

func resolvePosKey(root string, dirs map[string]bool, pos gotoken.Position) string {
	if pos.Filename == "" || pos.Line < 1 {
		return ""
	}
	name := pos.Filename
	if !filepath.IsAbs(name) {
		name = ""
		candidates := []string{root}
		for dir := range dirs {
			candidates = append(candidates, dir)
		}
		for _, dir := range candidates {
			if cand := filepath.Join(dir, pos.Filename); fileExists(cand) {
				name = cand
				break
			}
		}
		if name == "" {
			name = filepath.Join(root, pos.Filename)
		}
	}
	return filepath.Clean(name) + ":" + strconv.Itoa(pos.Line) + ":" + strconv.Itoa(pos.Column)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
