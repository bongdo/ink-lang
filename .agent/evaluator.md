# Ink evaluator

Check the task against design/SPEC.md, design/GAPS.md and design/AGENT.md.

Must be true:
- `./bin/check` is green, including tests/cases, tests/run.sh and the acceptance
  agent script.
- The gap is implemented in the canonical syntax; SPEC.md wins over old notes.
- `ink check --json` positions, codes and hints stay correct and tested.
- New behavior has conformance tests in tests/cases.
- GAPS.md / AGENT.md status rows reflect reality.
- No comments except machine directives and `shortcut:` notes.

End with `VERDICT: PASS` or `VERDICT: FAIL`.
