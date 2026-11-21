# Ink docs

Two internal layers, plus the public entry:

| Layer | Question | Where |
|---|---|---|
| Product | What do we ship, and what is the contract? | [`product/`](product/) |
| Maintenance | How is it built, and why was it decided? | [`maintain/`](maintain/) |
| Guides | How do I use it? | [`guides/`](guides/) |
| Archive | What did it look like before? | [`archive/`](archive/), including dated proposals |

## Product

| Document | Contains |
|---|---|
| [`product/SPEC.md`](product/SPEC.md) | normative language |
| [`product/PAR.md`](product/PAR.md) | normative `par` / `within` annex |
| [`product/FFI.md`](product/FFI.md) | Go boundary |
| [`product/FEATURES.md`](product/FEATURES.md) | the only status table |
| [`product/EDGES.md`](product/EDGES.md) | the only list of deviations |

## Maintenance

| Document | Contains |
|---|---|
| [`maintain/decisions.md`](maintain/decisions.md) | append-only decision log |
| [`maintain/IMPL.md`](maintain/IMPL.md) | compiler package contracts |

## Guides

- [`guides/getting-started.md`](guides/getting-started.md)
- [`guides/lsp.md`](guides/lsp.md)
- `ink guide` — the full embedded agent reference.

## Rules

- `product/SPEC.md` and `product/PAR.md` describe the language as it ships.
  If code and docs disagree, fix both in the same change.
- `product/FEATURES.md` is the only status table.
- `product/EDGES.md` is the only list of known deviations.
- `maintain/decisions.md` is append-only. Never edit an old entry.
- `archive/` is history. Do not link to it as current documentation.
- No gap numbers. An item is either a feature (status in `FEATURES.md`) or a
  deviation (entry in `EDGES.md`).
