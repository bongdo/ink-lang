package main

import (
	"fmt"
	"html"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bongdo/ink/internal/lexer"
	"github.com/bongdo/ink/internal/token"
)

type seg struct {
	start int
	end   int
	class string
	text  string
}

func kindClass(k token.Kind) string {
	switch {
	case k >= token.Use:
		return "kw"
	case k == token.String || k == token.RawString || k == token.Rune:
		return "str"
	case k == token.Int || k == token.Float || k == token.Duration:
		return "num"
	case k == token.Ident:
		return "id"
	case k == token.EOF:
		return ""
	default:
		return "op"
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: syntaxshot <file.ink> [from:to]")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	text := string(raw)
	name := os.Args[1]
	if len(os.Args) > 2 {
		parts := strings.SplitN(os.Args[2], ":", 2)
		if len(parts) == 2 {
			from, _ := strconv.Atoi(parts[0])
			to, _ := strconv.Atoi(parts[1])
			lines := strings.Split(text, "\n")
			if from >= 1 && to <= len(lines) && from <= to {
				text = strings.Join(lines[from-1:to], "\n")
			}
		}
	}

	tokens, comments, _ := lexer.Lex([]byte(text))
	var segs []seg
	for _, c := range comments {
		segs = append(segs, seg{c.Pos.Offset, c.End.Offset, "cm", text[c.Pos.Offset:c.End.Offset]})
	}
	for i, t := range tokens {
		if t.Kind == token.EOF {
			continue
		}
		class := kindClass(t.Kind)
		if t.Kind == token.Ident {
			r, _ := utf8.DecodeRuneInString(t.Lit)
			if unicode.IsUpper(r) {
				class = "ty"
			} else if i+1 < len(tokens) && tokens[i+1].Kind == token.LParen {
				class = "fn"
			}
		}
		segs = append(segs, seg{t.Pos.Offset, t.End.Offset, class, text[t.Pos.Offset:t.End.Offset]})
	}
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].start < segs[j].start })

	var b strings.Builder
	pos := 0
	for _, s := range segs {
		if s.start < pos || s.end > len(text) {
			continue
		}
		if s.start > pos {
			b.WriteString(html.EscapeString(text[pos:s.start]))
		}
		fmt.Fprintf(&b, `<span class="%s">%s</span>`, s.class, html.EscapeString(s.text))
		pos = s.end
	}
	b.WriteString(html.EscapeString(text[pos:]))

	base := name
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	lines := strings.Count(text, "\n") + 1
	height := 118 + lines*29

	fmt.Printf(`<!doctype html>
<html><head><meta charset="utf-8"><style>
  html, body { margin: 0; background: #101216; }
  body { padding: 24px; font-family: ui-monospace, "JetBrains Mono", "SF Mono", Menlo, monospace; }
  .card { background: #16181d; border: 1px solid #262a31; border-radius: 14px; overflow: hidden; }
  .head { display: flex; justify-content: space-between; align-items: center;
          padding: 16px 24px; border-bottom: 1px solid #22262d; }
  .file { color: #8b93a1; font-size: 13px; letter-spacing: .14em; text-transform: uppercase; }
  .badge { color: #d8a657; border: 1px solid #4a3b22; padding: 4px 12px; border-radius: 999px;
           font-size: 12px; letter-spacing: .12em; text-transform: uppercase; }
  pre { margin: 0; padding: 22px 24px; font-size: 17px; line-height: 29px; color: #c9d1d9; }
  .kw { color: #d8a657; } .ty { color: #7fb4ca; } .str { color: #a7c080; }
  .num { color: #d699b6; } .cm { color: #5f6672; font-style: italic; }
  .fn { color: #7aa2f7; } .op { color: #a9b1bd; } .id { color: #c9d1d9; }
</style></head><body>
<div class="card" style="width:1120px;height:%dpx">
  <div class="head"><span class="file">%s</span><span class="badge">Ink · 0.2</span></div>
  <pre>%s</pre>
</div>
</body></html>
`, height, html.EscapeString(base), b.String())
}
