package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bongdo/ink/internal/agent"
	"github.com/bongdo/ink/internal/config"
	"github.com/bongdo/ink/internal/driver"
	"github.com/bongdo/ink/internal/lsp"
	"github.com/bongdo/ink/internal/mcp"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "-C" {
		if err := os.Chdir(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "ink:", err)
			os.Exit(2)
		}
		args = args[2:]
	}
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "build":
		os.Exit(build(args))
	case "run":
		os.Exit(run(args))
	case "check":
		os.Exit(check(args))
	case "test":
		os.Exit(test(args))
	case "fmt":
		os.Exit(fmtCmd(args))
	case "lsp":
		os.Exit(serve(args))
	case "mcp":
		os.Exit(mcpCmd(args))
	case "new":
		os.Exit(newCmd(args))
	case "init":
		os.Exit(initCmd(args))
	case "guide":
		fmt.Print(agent.Guide)
	case "version", "-v", "--version":
		fmt.Println("ink", driver.Version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ink — the Ink compiler

usage:
  ink [-C dir] <command> ...
  ink build [-o output] [-embed dir] [go build flags]
  ink run [--race] [-embed dir] [-- program args]
  ink check [--json] [--fast] [--race]
  ink test [go test flags]
  ink fmt [-l] [-w] [--json] [paths]
  ink lsp [--root dir]
  ink mcp [--root dir]
  ink new <dir>
  ink init [dir]
  ink guide
  ink version

exit codes: 0 success, 1 diagnostics or failure, 2 usage
`)
}

func open() (*driver.Project, int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ink:", err)
		return nil, 1
	}
	p, err := driver.Open(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ink:", err)
		return nil, 1
	}
	return p, 0
}

func finish(p *driver.Project) int {
	if len(p.Errors) > 0 {
		p.PrintErrors(os.Stderr)
	}
	if p.HasErrors() {
		return 1
	}
	return 0
}

func build(args []string) int {
	embeds, args := takeEmbeds(args)
	out := ""
	var goArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-o" && i+1 < len(args) {
			out = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(a, "-o=") {
			out = strings.TrimPrefix(a, "-o=")
			continue
		}
		if a == "--" {
			goArgs = append(goArgs, args[i+1:]...)
			break
		}
		goArgs = append(goArgs, a)
	}
	p, code := open()
	if p == nil {
		return code
	}
	p.Embed = embeds
	if err := p.Build(out, goArgs); err != nil {
		if err == driver.ErrCheck {
			p.PrintErrors(os.Stderr)
			return 1
		}
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	return finish(p)
}

func dirArgBeforeSeparator(args []string) (string, bool) {
	for _, a := range args {
		if a == "--" {
			return "", false
		}
		if fi, err := os.Stat(a); err == nil && fi.IsDir() {
			return a, true
		}
	}
	return "", false
}

func flagErr(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		usage()
		return 0
	}
	usage()
	return 2
}

func run(args []string) int {
	if dir, ok := dirArgBeforeSeparator(args); ok {
		fmt.Fprintf(os.Stderr, "ink: %q is a directory; cd into the project and put program args after --\n", dir)
		return 2
	}
	embeds, args := takeEmbeds(args)
	race, progArgs, err := parseRunArgs(args)
	if err != nil {
		return flagErr(err)
	}
	p, code := open()
	if p == nil {
		return code
	}
	p.Embed = embeds
	code, err = p.Run(race, progArgs)
	if err != nil {
		if err == driver.ErrCheck {
			p.PrintErrors(os.Stderr)
			return 1
		}
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	if p.HasErrors() {
		p.PrintErrors(os.Stderr)
		return 1
	}
	return code
}

func takeEmbeds(args []string) ([]string, []string) {
	var embeds, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-embed" && i+1 < len(args) {
			embeds = append(embeds, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(a, "-embed=") {
			embeds = append(embeds, strings.TrimPrefix(a, "-embed="))
			continue
		}
		rest = append(rest, a)
	}
	return embeds, rest
}

func parseRunArgs(args []string) (bool, []string, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	race := fs.Bool("race", false, "build with the race detector")
	if err := fs.Parse(args); err != nil {
		return false, nil, err
	}
	return *race, fs.Args(), nil
}

func check(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print diagnostics as JSON")
	fast := fs.Bool("fast", false, "parse and check without the generated Go typecheck")
	race := fs.Bool("race", false, "typecheck with the race detector")
	if err := fs.Parse(args); err != nil {
		return flagErr(err)
	}
	p, code := open()
	if p == nil {
		return code
	}
	if err := p.Check(driver.CheckOptions{Tidy: true, Typecheck: true, Fast: *fast, Race: *race}); err != nil {
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	if !*fast && !p.HasErrors() {
		if _, err := os.Stat(filepath.Join(p.Root, "zz_ink.go")); err == nil {
			_ = p.Generate(false)
		}
	}
	if *asJSON {
		if err := p.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "ink:", err)
			return 1
		}
		for _, e := range p.Errors {
			if !e.Warning {
				return 1
			}
		}
		if *fast {
			fmt.Fprintln(os.Stderr, "ink: fast check is clean; the generated-Go typecheck and go vet were skipped, run ink check to finish")
		}
		return 0
	}
	status := finish(p)
	if status == 0 && *fast {
		fmt.Fprintln(os.Stderr, "ink: fast check is clean; the generated-Go typecheck and go vet were skipped, run ink check to finish")
	}
	return status
}

func mcpCmd(args []string) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", "", "project root")
	if err := fs.Parse(args); err != nil {
		return flagErr(err)
	}
	dir := *root
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "ink:", err)
			return 1
		}
		dir = cwd
	}
	found, err := config.FindRoot(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	if err := mcp.Run(os.Stdin, os.Stdout, found); err != nil {
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	return 0
}

func test(args []string) int {
	embeds, args := takeEmbeds(args)
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	p, code := open()
	if p == nil {
		return code
	}
	p.Embed = embeds
	code, err := p.Test(args)
	if err != nil {
		if err == driver.ErrCheck {
			p.PrintErrors(os.Stderr)
			return 1
		}
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	if p.HasErrors() {
		p.PrintErrors(os.Stderr)
		return 1
	}
	return code
}

func fmtCmd(args []string) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	write := fs.Bool("w", false, "write result to source files")
	list := fs.Bool("l", false, "list files whose formatting differs")
	asJSON := fs.Bool("json", false, "print differing paths as JSON")
	if err := fs.Parse(args); err != nil {
		return flagErr(err)
	}
	if err := driver.Format(*write, *list, *asJSON, fs.Args(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	return 0
}

func serve(args []string) int {
	fs := flag.NewFlagSet("lsp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", "", "project root")
	if err := fs.Parse(args); err != nil {
		return flagErr(err)
	}
	dir := *root
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if err := lsp.Run(os.Stdin, os.Stdout, os.Stderr, dir); err != nil {
		fmt.Fprintln(os.Stderr, "ink lsp:", err)
		return 1
	}
	return 0
}

func newCmd(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: ink new <dir>")
		return 2
	}
	if err := driver.New(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	fmt.Println("created", args[0])
	return 0
}

func initCmd(args []string) int {
	dir := "."
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "usage: ink init [dir]")
		return 2
	}
	if len(args) == 1 {
		dir = args[0]
	}
	if err := driver.Init(dir); err != nil {
		fmt.Fprintln(os.Stderr, "ink:", err)
		return 1
	}
	fmt.Println("initialized", dir)
	return 0
}
