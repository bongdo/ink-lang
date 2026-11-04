# `check --json` repeats the same diagnostic

## Problem

`ink check --json` can emit the same diagnostic twice, with the same file,
span, code and message. An agent that lists findings, counts them or diffs two
runs sees two problems where there is one, and the LSP publishes the duplicate
too, because both read `sema.Program.Errors`.

```
$ cat main.ink
fn main() void or error {
    var p: *Missing = nil;
    return;
};

$ ink check --json
[{"file":".../main.ink","line":2,"col":12,"endLine":2,"endCol":13,
  "severity":"error","code":"semantic","message":"unknown type Missing","hint":""},
 {"file":".../main.ink","line":2,"col":12,"endLine":2,"endCol":13,
  "severity":"error","code":"semantic","message":"unknown type Missing","hint":""}]
```

The typed-`var` path resolves the declared type once for the pointer element
and once for the `nil` check, so `errorf` is called twice with the same
arguments. Nothing downstream deduplicates: `driver.Diagnostics` maps every
`sema.Error` to one array element, and the LSP maps them to diagnostics.

## Smallest change

Deduplicate where the error is recorded: keep a `map[string]bool` on
`sema.Program`, keyed by `path + pos.Offset + msg`, and drop an identical
`sema.Error` in `Program.errorf` (`internal/sema/sema.go`). Warnings take the
same path if `warnf` is folded in; not required for the first patch.

This fixes the CLI, the LSP and `ink test` output at once, without teaching
each consumer the same filter. Two diagnostics that share a path, an offset and
a message are indistinguishable to every consumer, so nothing is lost.

## Acceptance check

- The program above returns exactly one element from `ink check --json`, exit
  code 1; `ink check` prints one line.
- A sema unit test asserts one error for a typed `var` with an unknown type,
  including the pointer form (`*Missing`).
- `examples/problems` gets this program so the harness's exact-count rule
  (currently 15 diagnostics) protects the fix; a regression fails with
  `want N diagnostics, got N+1`.
- The LSP test that publishes diagnostics for this buffer contains one
  diagnostic, not two.

## Risks

- The key ignores `Warning` and `severity`. If the same message is ever both a
  warning and an error at one offset, one is dropped; no current rule does
  this, and the key can include the flag if it ever does.
- Deduplication hides a second *intentional* report of the same problem at the
  same byte. If a later feature wants per-context errors (for example one per
  generic instantiation), it needs a distinct message or position.
- This is a lid on the symptom. The true fix is to resolve the declared type
  once in the typed-`var` path; the dedupe is cheap, safe and does not block
  that cleanup.
