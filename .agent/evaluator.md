# Ink evaluator

Check the task against docs/SPEC.md, docs/FEATURES.md and docs/EDGES.md.

Must be true:
- `./bin/check` is green, including tests/cases, tests/run.sh and the acceptance
  agent script.
- The change is implemented in the canonical syntax; SPEC.md wins over old notes.
- `ink check --json` positions, codes and hints stay correct and tested.
- New behavior has conformance tests in tests/cases.
- FEATURES.md status and EDGES.md entries reflect reality.
- No comments except machine directives and `shortcut:` notes.

End with `VERDICT: PASS` or `VERDICT: FAIL`.
