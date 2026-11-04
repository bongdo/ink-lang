# Decisions

## 2026-10-27 — Compile to Go and let the Go type-checker validate

**Контекст:** Ink targets Go's runtime and stdlib; implementing a complete
type-checker twice would duplicate `go/types`.
**Решение:** sema resolves names, imports, enums, `or`, `defer`, context and
`par`; codegen emits Go; `go build` validates the remaining types. Generated
code carries `//line` directives so errors point to `.ink` files.
**Почему:** one source of truth for types, full Go interop from day one.
**Что отвергли:** a standalone Ink type-checker and a non-Go backend.
**Цена:** diagnostics for Go type errors need a build, and generated code must
be clean enough not to add errors.

## 2026-10-27 — Enums are sealed interfaces with pointer variant structs

**Контекст:** sum types need exhaustive `match` and recursive variants.
**Решение:** `type Shape interface { _s_isShape() }`; each variant is a struct
`ShapeCircle` with a pointer receiver marker; construction wraps the pointer in
the enum interface; `match` is a type switch.
**Почему:** recursion works without boxing every field, `match` is a normal Go
type switch, and the marker seals the type.
**Что отвергли:** tagged structs and generated variant closures.
**Цена:** an enum cannot be zero-initialized; variants must be constructed.

## 2026-10-27 — `or` is statement-level

**Контекст:** `or` needs to run statements (`or return`, `or err { ... }`) and
bind an error.
**Решение:** `or` may be the whole initializer, right side, expression
statement, or return value; it cannot be nested inside another expression.
**Почему:** codegen is a straight-line prelude; no expression-level control
flow, no hidden temporaries in arbitrary places.
**Что отвергли:** `or` as a general postfix operator.
**Цена:** `f(g() or return)` must be split into two statements.

## 2026-10-27 — Block-scoped defer via an explicit stack

**Контекст:** `defer` must fire at the end of the nearest block, including
through `return`, `break`, `continue`, `or return` and panic.
**Решение:** every function with defers owns a `inkrt.DeferStack`; `defer`
pushes a closure; block exits call `RunTo(mark)`; panic safety comes from one
function-level `defer RunTo(0)`.
**Почему:** conditional defers work, loops do not leak, and no closure frame
signals are needed.
**Что отвергли:** function-scoped defer and a closure per deferred block.
**Цена:** one slice of closures per function that uses defer.

## 2026-10-27 — Hidden context is computed by a call-graph fixpoint

**Контекст:** `par` and `within` need cancellation, but `context.Context` must
not appear in Ink code.
**Решение:** a function receives a hidden `_s_ctx` parameter only when it or a
callee needs one; Go functions whose first parameter is `context.Context` get
it automatically; `within` rebinds it to a deadline context.
**Почему:** pure Ink stays context-free without coloring every function.
**Что отвергли:** context on every function and explicit `ctx` parameters.
**Цена:** a context-needing function exposes the context at the Go FFI
boundary, and callbacks passed to Go cannot need one.

## 2026-10-27 — One compiler binary, no external tools at runtime

**Контекст:** build, run, test, format and LSP need the same parser and sema.
**Решение:** `cmd/ink` contains all commands; `ink build` writes Go and
execs `go build`; the LSP server is in the same binary and uses parser/sema
directly.
**Почему:** one install, one version, no plugin drift.
**Что отвергли:** a separate language server binary and a Node-based client
for the core.
**Цена:** the binary is larger and `go` must be installed to build projects.

## 2026-10-27 — pero-inked is a focused port, not a byte-for-byte rewrite

**Контекст:** pero is 27k lines of Go built on Bubble Tea; Ink v1 has no
multi-return tuples, no `any` type switches, and no interface inheritance.
**Решение:** port the editor core (`internal/ed`), config, themes, a tcell UI,
search, undo and LSP diagnostics; use small Go shims for terminal events and
the background LSP transport.
**Почему:** the result is a working editor that exercises the language, not a
demo that cannot build.
**Что отвергли:** a mechanical 12k-line UI translation, Bubble Tea, and a
JavaScript-only editor.
**Цена:** no AI/ghost text, soft wrap, system clipboard or full LSP UI yet.

## 2026-10-27 — One canonical way per intent; Go is a foreign layer

**Контекст:** Go interop leaked method sets, context wrappers, Go types and
feature-parity work into the language surface. That creates the same
"many ways to do it" problem Go itself is criticized for.
**Решение:** app code has one canonical form per intent; Go is a foreign/FFI
layer. Complex Go libraries go through Ink facades or adapters; raw Go is an
escape hatch. Simple imports are bare (`use fmt;`), struct literals are named,
propagation is `or return`, comments are `//` only.
**Почему:** external libraries stay available, while app code remains small.
**Что отвергли:** Go feature parity (full generics constraints, generic enums,
expression-level `or`, user variadics, doc comments, full IDE features) and
hidden context across arbitrary Go callbacks.
**Цена:** binding/facade code for complex libraries; some Go APIs are not
directly usable from app code.

## 2026-10-27 — `ink check` checks project tests

**Контекст:** a type error in a `test` block was only caught by `go test`.
**Решение:** `ink check` generates `zz_ink_test.go` in memory and enables
`packages.Config.Tests`; LSP stays production-only for now.
**Почему:** `check` should validate the whole project before build/run.
**Что отвергли:** a separate `check --tests` switch; tests are part of validity.
**Цена:** cold `ink check` is slower because it runs `go list -test`.

## 2026-10-29 — Синтаксические фиксы 0.2: массивы, `or`, канон форматтера

**Контекст:** `T[N]` с именованной константой падал для не-builtin типов,
multi-result `A, B or error` нечем было пробросить, а форматтер и примеры
расходились по `void`, `fn` в интерфейсах и отступам `case`.
**Решение:** `T[N]` остаётся постфиксным, но sema превращает не-дженерик
`T[const]` в массив; `or` пробрасывает все не-error результаты; канон
форматтера — явный `void`, интерфейсные методы без `fn`, многострочные
composite literals сохраняются; `ink fmt -l` входит в `bin/check`.
`break`/`continue` внутри цикла в `par` разрешены, через границу — нет.
**Почему:** это убирает дыры в языке и делает канон проверяемым, не меняя
базовую форму `T[N]` и statement-level `or`.
**Что отвергли:** `[N]T` как единственный синтаксис массива; expression-level
`or`; `if err != nil { return err }` как проброс для multi-result.
**Цена:** `or` теперь генерирует несколько временных значений; `T[N]` с
константой из Go-пакета не поддержан; formatter drift надо удерживать через
`fmt -l`.

## 2026-10-30 — Каноническая поверхность: объявления, `const`, typed `var`, `match`, `par`

**Контекст:** старые формы с `=` перед телом типа или функции дублировали
канонические; `const` вёл себя как runtime-слот; `nil` требовал
`nil[T]` там, где тип и так известен; `break` в `match` и `void` были
недоделаны, а `par for` по map давал недетерминированный победитель ошибки.
**Решение:** `type X struct/enum/interface` и `fn f() R { ... }` — канон,
старые формы с `=` перед телом отклоняются, `=` остаётся только у alias;
`const` только package-level, без типа, как compile-time имя; typed `var`
(`var p: *T = nil`) даёт expected type для bare `nil`, `nil[T]` — escape hatch
для `any`/interface; `break`/`continue` в `match` метят цикл; `par for` по map
и `for ()` в `par` запрещены; `void` разрешён только как результат; путь с
`/v2` требует alias; `_` остаётся единственным discard, без `discard`.
**Почему:** одна форма на намерение, ошибки видны до codegen, а `par` не
обещает детерминизм там, где его нет.
**Что отвергли:** `discard`-builtin; `nil[T]` как default; map в `par for` с
ord `-1`; `void` как обычный тип; alias-less versioned import.
**Цена:** старые формы с `=` перед телом больше не парсятся — их надо
переписать вручную на канон; `ink fmt` форматирует только канонический
ввод. `const` больше нельзя объявить локально; map-параллелизм требует
промежуточного slice.

## 2026-10-30 — Agent readiness is priority 1, 1.0 polish deferred

**Контекст:** язык и тулинг уже работают, но агенту не хватает дешёвого
feedback loop: машиночитаемых диагностик, точных колонок, быстрого check,
agent guide и LSP-рефакторингов.
**Решение:** `design/AGENT.md` задаёт P0 (agent contract, JSON diagnostics,
columns, fast check, examples), P1 (LSP references/rename/code actions), P2
(`-race` path, G10/G11 docs), P3 (1.0 polish). 1.0-фичи не начинаются, пока
P0 не готов.
**Почему:** без надёжного цикла «правка → точная ошибка → фикс» агент угадывает;
1.0-полировка не улучшает эту петлю.
**Что отвергли:** 1.0-first порядок из старого `GAPS.md`; новый язык и stdlib
до готовности агентского контура.
**Цена:** 1.0 откладывается; зато P0 в основном тулинг и документация, а не
изменения семантики.

## 2026-10-30 — Agent contract and diagnostics

**Контекст:** агенту нужен был цикл «правка → точная ошибка → фикс» без
чтения исторических заметок: машиночитаемые диагностики с кодами и подсказками,
быстрый check, гид в репозитории и в бинаре, LSP-рефакторинги и документированный
race path.
**Решение:** корневой `AGENTS.md` встроен в бинарь (`internal/agent/guide.md`),
печатается `ink guide` и ставится `ink new`/`ink init` (init не
перезаписывает существующий файл). `ink check --json` печатает массив с
`file`, `line`, `col`, `endLine`, `endCol`, `severity`, `code`, `message`,
`hint`; позиции 1-based, чистый вывод `[]`, ошибки дают exit 1. `--fast`
останавливается после parse/resolve/sema, `--race` включает race при
typecheck; `ink run -race` и `ink test -race` пробрасывают флаг в Go.
`ink fmt -l --json` печатает массив путей. Коды и hints стабильны в
`internal/diag`. LSP получил references/rename для объявлений проекта, code
actions для канонических ошибок, диагностику `test`-блоков, debounce 150 ms для
typecheck, очистку буфера на `didClose` и workspace symbols. G10/G11 описаны в
`AGENTS.md` и `examples/limits`.
**Почему:** агент чинит ошибки по коду и позиции, а не по тексту; один
embedded гид исключает дрейф между репозиторием и бинарём; Go остаётся
оракулом для типов и гонок.
**Что отвергли:** auto-apply фиксов в CLI (решение остаётся за агентом или
редактором); полный LSP IDE parity (rename символов Go-библиотек, проверка
конфликтов имён); runtime-инструментацию гонок в компиляторе.
**Цена:** A2 частичный: концы lexer/parser точные, у sema/typecheck — один
символ, пути в `//line` абсолютные (G34). `nil-untyped` фикс консервативный,
rename не проверяет конфликты. Sema в LSP всё ещё синхронна; `check --race`
только компилирует с race, но не запускает программу. Документация должна
обновляться вместе с кодами и флагами.

## 2026-10-30 — Short ambient guide, on-demand reference, OpenCode LSP

**Контекст:** полный гайд в `AGENTS.md` загружался в каждый запрос, хотя агенту
обычно нужны только команды и диагностики; OpenCode умеет сам подмешивать LSP-
ошибки после `edit`/`write` и форматировать файл.
**Решение:** корневой `AGENTS.md` — короткий контракт (цикл, JSON, layout,
generated-файлы); полный справочник остаётся в `internal/agent/guide.md`,
печатается `ink guide` и лежит в `.opencode/skills/ink/SKILL.md`, который
OpenCode грузит по требованию. `opencode.json` запускает `ink lsp` для `.ink`
и `ink fmt -w $FILE` после правок. `ink new`/`init` ставят короткий
`AGENTS.md`.
**Почему:** OpenCode редактирует через `edit`/`write`/`patch`, а LSP — это
diagnostics и навигация; полный гайд не должен занимать ambient-контекст.
**Что отвергли:** MCP-обёртку над `check`/`fmt` и реализацию call hierarchy в
LSP сейчас; auto-apply фиксов в CLI.
**Цена:** OpenCode без `opencode.json` не запускает LSP; LSP-диагностики после
`edit` показывают только errors, не warnings; code actions/rename остаются для
редакторов, не для агента OpenCode. В этом репозитории перед LSP нужен
`bin/build`.

## 2026-11-04 — Relative `//line` paths and remaining gap closure

**Контекст:** G18, G23, G29, G34 и G46 были последними языковыми и
диагностическими хвостами 0.2 preview.
**Решение:** sema резолвит явные type arguments вызова и подставляет их в
param/result типы; `s[i]` документирован как `byte`; пакетные переменные
инферятся до неподвижной точки с ошибкой на цикл; `//line` печатает путь
относительно сгенерированного файла, а typecheck разбирает относительные пути
и bundle-заголовки тестовых пакетов; formatter оставляет trailing-комментарии
на своей строке.
**Почему:** относительные пути не текут в артефакты и переживают перенос
проекта; byte-индексация совпадает с Go; fixpoint убирает слабые типы у
forward-ссылок; комментарии не должны переезжать.
**Что отвергли:** менять `s[i]` на rune; эвристики конца для sema/typecheck без
токена; перенос sema в LSP-воркер (G43) в этом проходе.
**Цена:** концы sema/typecheck остаются в один символ (G34); нет wrapping в
formatter; sema в LSP синхронна (G43).

## 2026-11-04 — Regression fixes for G18, G29 and G46

**Контекст:** независимая проверка нашла, что явные type args у Go generic
вызовов отвергались,typed-циклы проходили `check --fast`, а formatter не был
идемпотентен и терял `use`/`match`/top-level trailing-комментарии.
**Решение:** Go generic вызовы проверяют арность через `go/types` и принимаются;
граф инициализации строится по всем пакетным `var`, включая объявленный тип, и
не спускается в тела function literal; formatter снимает все лишние переводы
строк перед trailing-комментарием и сбрасывает комментарии перед закрытием
`match`.
**Почему:** explicit args должны работать одинаково для Ink и Go; типизированный
цикл всё равно остаётся циклом инициализации; один `fmt -w` обязан быть
неподвижной точкой.
**Что отвергли:** отказываться от Go generic args как от неподдерживаемого
случая; считать объявленный тип доказательством отсутствия цикла.
**Цена:** sema не подставляет Go type params в hover-типы; G43 остаётся
синхронной; концы sema/typecheck по-прежнему в один символ.
