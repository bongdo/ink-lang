# Ink docs

Read by question, not by filename:

| Question | Document |
|---|---|
| What is Ink and how do I start? | [`../README.md`](../README.md), [`getting-started.md`](getting-started.md) |
| What works today? | [`FEATURES.md`](FEATURES.md) — status per area |
| Where can the language surprise me? | [`EDGES.md`](EDGES.md) — ranked, with workarounds |
| What is the language? | [`SPEC.md`](SPEC.md) — normative |
| What exactly does `par` do? | [`PAR.md`](PAR.md) — normative concurrency annex |
| How does Ink talk to Go? | [`FFI.md`](FFI.md) |
| Why is there only one spelling? | [`ONE_WAY.md`](ONE_WAY.md) |
| How is the compiler built? | [`impl/IMPL.md`](impl/IMPL.md), [`impl/TYPECHECK.md`](impl/TYPECHECK.md) |
| Why was this decided? | [`decisions.md`](decisions.md) — append-only |
| What was proposed? | [`proposals/`](proposals/) — dated, not normative |
| What did the project look like before? | [`archive/`](archive/) — frozen, not maintained |
| Slides and cheat sheet | [`presentation/`](presentation/) — 0.2 snapshot, not normative |
| Editor setup | [`lsp.md`](lsp.md) |

## Rules

- `SPEC.md`, `PAR.md` and `FFI.md` describe the language as it ships. If code
  and docs disagree, that is a bug; fix both in the same change.
- `FEATURES.md` is the only status table. Do not add status to other documents.
- `EDGES.md` is the only list of known deviations. Open items there are the
  worklist; accepted items are not re-litigated.
- `decisions.md` is append-only. Never edit an old entry; add a new one.
- `archive/` is history. Do not link to it as current documentation.
- No gap numbers. An item is either a feature (status in `FEATURES.md`) or a
  deviation (entry in `EDGES.md`).
