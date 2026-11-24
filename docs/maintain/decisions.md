# Decisions

Links in entries are historical. Removed documents are in the VCS history;
current paths are in [`../README.md`](../README.md).

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

## 2026-11-04 — IIFE init cycles and G23 oracle

**Контекст:** ре-проверка показала, что G23 не имел conformance-оракула, а
`tests/cases/strings` проходит и на старом бинаре; правило «function literal
bodies не создают init-зависимостей» было неверным для immediately invoked
literals.
**Решение:** граф инициализации обходит тело function literal только когда
литерал является callee вызова; добавлен `forward_iife` с проверкой `--fast`.
Для G23 добавлены positive/negative кейсы `string_index_byte` и
`string_index_rune` (full check, точная позиция), а sema-оракулом остаётся
`TestStringIndexIsByte`.
**Почему:** IIFE исполняется во время инициализации, как в Go; вывод байта в
рантайме не доказывает тип в sema.
**Что отвергли:** считать `tests/cases/strings` регрессией G23; добавлять
общую проверку присваиваемости только ради этого кейса.
**Цена:** циклы через свободные функции по-прежнему ловит только полный Go
typecheck; `--fast` их не видит.

## 2026-11-19 — Documentation is organized by question, not by work item

**Контекст:** the project had two competing documentation trees (`design/` and
`docs/`) and eleven design files of five different kinds. The 0.2 gap list
mixed status, roadmap, invariants, non-goals and decisions; `G49` was a
deliberate non-goal, not a gap. Sources drifted: `TYPECHECK.md` still said sema
ran synchronously after the analysis worker shipped, `SPEC.md` pointed at the
gap list as its status, and `README.md` duplicated both.
**Решение:** one live tree in `docs/`: `SPEC.md`, `PAR.md`, `FFI.md` and
`ONE_WAY.md` are normative; `FEATURES.md` is the only status table; `EDGES.md`
is the only list of deviations, marked open or accepted; `impl/` holds compiler
contracts; `archive/` holds frozen history; `decisions.md` stays append-only.
No gap or roadmap numbers.
**Почему:** a reader has one place to ask "does it work", "will it surprise
me" and "what is the rule". Work comes from open `EDGES.md` entries and vault
features, not from a numbered backlog maintained in prose.
**Что отвергли:** keeping `GAPS.md` and layering new files on top; a status
file per area; rewriting archived notes into the new format.
**Цена:** old `G*`/`A*` ids are gone and archived notes still mention them;
links to `design/` break once, with no redirect layer.

## 2026-11-20 — Error surface: `T!`, `fail`, bare imports, `:` literals

**Контекст:** 0.2 used `T or error`, `or return`, quoted paths for multi-segment
imports, `field = value` literals, and an arbitrary name in `or <name> { }`.
The same word `or` appeared in types and handlers; `return` hid the error;
imports had two lexical forms; literals looked like assignments.
**Решение:** fallible results are `T!`, `(A, B)!` or `void!`. Handlers are
`or fail;`, `or fail("context");`, `or err { ... }` and `or panic(...)`, always
starting on a new line. The handler name is always `err`. `fail(err, "context")`
wraps and returns. Import paths are bare. Struct, enum and map literals use `:`.
**Почему:** one meaning per word and one lexical form. The error is always named
or returned, never bound invisibly.
**Что отвергли:** `Config?` (reads as nil), `Result[Config]` (heavier error
model), keeping `or return` as an alias, quoting every import.
**Цена:** every `or error`, `or return`, quoted import and `=` literal had to be
migrated once; `fail` is reserved; the formatter rewrites all handlers.

## 2026-11-21 — Project file is `ink.conf`, not TOML

**Контекст:** `BurntSushi/toml` parsed three keys (`name`, `module`, `go`). It
was the only reason for the dependency and the file format implied full TOML
support that the compiler never used.
**Решение:** `ink.conf` is a flat `key = "value"` file parsed with the standard
library. `ink.toml` is still read when present. `ink new` and `ink init` write
`ink.conf`.
**Почему:** one less dependency, one less format to explain, no behavior change
for the three keys.
**Что отвергли:** keeping the TOML library for three keys; JSON; a full INI
parser.
**Цена:** two filenames during the transition; old projects keep working.

## 2026-11-21 — Docs split into product and maintenance

**Контекст:** live docs mixed the language contract, status, deviations,
decision history, compiler internals and proposals in one directory. `ONE_WAY`
described a 1.0 concept and contradicted `SPEC`; `TYPECHECK` duplicated `IMPL`.
**Решение:** `docs/product/` holds SPEC, PAR, FFI, FEATURES and EDGES;
`docs/maintain/` holds decisions and IMPL; `docs/guides/` holds user guides;
dated proposals, `ONE_WAY` and the presentation are frozen in `docs/archive/`.
`TYPECHECK` was merged into `IMPL` and deleted.
**Почему:** a reader knows whether a file is a promise, a tool, or history.
`vault/` remains the user's declaration layer and is not written by agents.
**Что отвергли:** a vault scaffold written by the agent; keeping proposals live;
a docs site.
**Цена:** old links to `docs/SPEC.md` and `docs/decisions.md` break once; the
archive keeps the old text.

## 2026-11-21 — Arenas are a block with an implicit handle

**Контекст:** Go 1.27 exposes `arena` behind `GOEXPERIMENT=arenas`, but Ink had
no way to allocate a batch and free it at once.
**Решение:** `arena a { ... };` owns one `arena.NewArena()` and frees it at block
exit, including return, break, continue and panic. `a.New[T]()` returns `*T`;
`a.MakeSlice[T](n[, cap])` returns `[]T`. `par` inside is rejected. The build,
run, test and typecheck paths set `GOEXPERIMENT=arenas` when a source uses one.
**Почему:** block scope makes the free point explicit and avoids exposing a
handle type or a `Free` call to the user.
**Что отвергли:** a first-class arena type; manual `a.Free()`; allowing `par`
inside an arena.
**Цена:** arena values must not escape the block. Using one after the block is
use-after-free; serde/sema does not prove escape. `arena` remains experimental
in Go.

## 2026-11-21 — Outdated docs are deleted, not archived

**Контекст:** the archive still carried `ONE_WAY`, old gap notes, four proposals
and the 0.2 presentation. The user asked to remove all outdated documents.
**Решение:** `docs/archive/` is gone. `ONE_WAY` was replaced by the vault note
`syntax`; old proposals were removed; the presentation was removed with its
assets. `decisions.md` keeps the history that matters.
**Почему:** an archive that nobody reads is still a second source of truth and
a file to maintain.
**Что отвергли:** keeping a frozen archive; moving the presentation to another
directory.
**Цена:** old links to `docs/archive/...` break; the removed text is in the VCS
history.

## 2026-11-21 — `go.mod` is the only project file

**Контекст:** `ink.conf` duplicated `module` and `go` from `go.mod` and existed
only to hold `name` and mark the root.
**Решение:** `go.mod` is the project marker and the source of module path and Go
version. `name` is the directory basename; `ink build -o` overrides the binary.
`ink new` writes `go.mod`; `ink init` creates one if absent. Old `ink.conf` and
`ink.toml` are no longer read.
**Почему:** one file, one source of truth, no custom format or parser.
**Что отвергли:** keeping `ink.conf` for `name`; a hidden project marker.
**Цена:** renaming the directory changes the default binary name; old projects
without `go.mod` need `ink init` or `ink build` once.

## 2026-11-21 — Arenas use an invisible allocator

**Контекст:** the first arena implementation exposed `arena a { a.New... }`:
a handle name plus two methods. The user asked for the smallest cognitive load.
**Решение:** `arena { ... }` has no handle. Inside the block `new[T]()` and
`make[T](n[, cap])` allocate from the arena; outside they are normal Go
allocations. Free is at block exit; `par` inside is rejected.
**Почему:** one keyword and one rule; `new` and `make` are already familiar.
**Что отвергли:** a named arena; `arena.New` as a namespace; arena on a function.
**Цена:** arena values must not escape the block; no escape analysis yet.

## 2026-11-21 — Arena values are checked at the block boundary

**Контекст:** the invisible allocator made `new`/`make` arena-backed, but a
pointer could be assigned to an outer variable, returned, or passed to a
function and used after `Free`.
**Решение:** sema tracks arena values. It rejects assignment to a captured
variable, `return`, function arguments, and `&T{}` inside an arena. Copying a
scalar field out (`n = p.X`) is allowed. `par` inside an arena is rejected.
**Почему:** the common escapes are caught without a full escape analysis or a
new arena type in every signature.
**Что отвергли:** documentation only; an `arena T` type marker; a runtime
liveness wrapper.
**Цена:** passing an arena pointer to a helper inside the block is rejected
conservatively; a value passed through a Go function or a captured container
can still leak.

## 2026-11-22 — Arena: parallel fill, allocations before par, calls borrow

**Контекст:** сборка кадра пишет пиксели в буфер, выделенный в арене, и хочет
заполнять строки параллельно; прежний запрет `par` внутри `arena` это
блокировал.
**Решение:** `par` и `par for` внутри арены разрешены. `new`/`make` внутри
воркера, использующего внешнюю арену, отвергаются: аллокация должна быть до
`par`. Воркер может открыть свою `arena { ... };`. Арены вкладываются, free идёт
изнутри наружу. Аргумент-арена в вызове — заимствование на время вызова, без
аннотаций.
**Почему:** запрещается только реальная гонка (общий handle Go-арены), а не
параллельная работа с уже выделенной памятью.
**Что отвергли:** полный запрет `par` — режет главный сценарий; heapify при
escape — тихо теряет арену там, где она нужна (загрузка кадра каждый кадр);
`borrow`-модификатор — новая поверхность ради той же семантики.
**Цена:** вызываемый может сохранить указатель и уронить программу после
`Free`; это документировано, статически не проверяется.

## 2026-11-22 — Arena memory never crosses a function boundary

This replaces the borrow-by-call entry above.

**Контекст:** разрешение передавать арену в вызов оставило дыру: вызываемый
мог сохранить указатель, и блок освобождал память под ним.
**Решение:** память арены не покидает блок ни в каком виде: ни присваиванием
наружу, ни `return`, ни аргументом или получателем вызова, ни `defer`, ни
`&T{}`. Читать можно поля, индексы, `len`/`cap`/`min`/`max`, копии структур и
массивов без ссылок, и `copy` наружу при скалярном элементе.
**Почему:** это единственное правило без анализа и без аннотаций, при котором
use-after-free невозможен по построению.
**Что отвергли:** borrow-аннотации и межпроцедурный escape-анализ (пользователь
отказался); heapify при escape (Go-арена не гарантирует, что утёкший указатель
держит память живой).
**Цена:** helper не может принять `*Frame` или аренный слайс; заполнение
инлайнится в тело воркера, а данные наружу идут через `copy`.

## 2026-11-22 — Arena warnings: fixed size in a loop, one chunk per iteration

**Контекст:** замеры 2.5K/240 кадров показали, что арена на кадр даёт 81 цикл GC
и p50 11.1 ms, а буфер, выделенный до цикла, — 2 цикла и 10.2 ms. Go считает
аренные аллокации обычными для пейсера, а освобождённый чанк возвращает в
оборот только после GC.
**Решение:** `ink check` предупреждает (`arena-loop`), если `arena` стоит в
последовательном цикле и все её `new`/`make` имеют константный размер. Если
блок за итерацию больше одного чанка Go (8 MB), предупреждение называет число
чанков и объясняет про GC.
**Почему:** это два случая, где арена заведомо проигрывает переиспользуемому
буферу; предупреждение стоит одну строку в отчёте и не ломает сборку.
**Что отвергли:** ошибку вместо предупреждения; автоматический хойстинг арены в
буфер (меняет семантику и требует нулевой инициализации каждый кадр);
манипуляции с `GOGC` (это не измерение и не язык).
**Цена:** предупреждение консервативно: если размер зависит от данных, оно
молчит, даже когда арена не нужна; 8 MB — внутренняя константа Go.

## 2026-11-23 — Arena backend: reusable typed scratch, no Go arena

This replaces the arena backend entries of 2026-11-21 and 2026-11-22. The
`arena-loop` and chunk warnings from 2026-11-22 are removed: with a reused
buffer a fixed-size arena in a loop is the intended shape.

**Контекст:** замеры 2.5K показали, что Go-арена не даёт «зоны без GC»: 81 цикл
на 240 кадров, полный чанк после `Free` ждёт GC в карантине, аллокации идут
через пейсер. `GOEXPERIMENT=arenas` — эксперимент на паузе.
**Решение:** `new`/`make` внутри `arena` берут память из `rt.Scratch[T]` —
пула типизированных batch-ей на тип в пакете; batch bump-аллоцирует, на выходе
из блока чистится и возвращается в пул. Go-арена и `GOEXPERIMENT` не
используются, память блока остаётся зарезервированной.
**Почему:** после прогрева аллокаций нет, `heap_live` не растёт, GC не
запускается (2.5K: 3 цикла вместо 81; 480p: 0). Escape-правило доказывает, что
буфер мёртв на выходе, поэтому reuse корректен.
**Что отвергли:** байтовый слот с `unsafe` (выигрыш не доказан, риск тихой
порчи); heapify при escape; `GOGC`-манипуляции как способ «не собирать».
**Цена:** память блока не возвращается; `new`/`make` в `par` по-прежнему
запрещены, потому что воркер делит batch с циклом; p50 10.9 ms против 8.4 ms у
спайка — разница в форме кода, не в бэкенде.

## 2026-11-23 — Arena buffer lifetime is the enclosing loop

**Контекст:** переиспользование в кадровом цикле требует, чтобы буфер пережил
одно выполнение блока; при этом память должна освобождаться, когда арена больше
не может выполниться.
**Решение:** scratch-переменные — локальные в функции. Арена прямо в цикле:
объявление до цикла, `Release()` после цикла. Арена вне цикла: объявление перед
блоком и `Release()` на выходе из блока.
**Почему:** одноразовая арена освобождает память сразу, как и ожидается; арена в
цикле не аллоцирует ни на одной итерации и освобождается, когда цикл кончился.
**Что отвергли:** держать буфер до конца программы; освобождать на каждом выходе
из блока (это аллокация на кадр, замер: 81 цикл GC).
**Цена:** арена внутри функции, которую вызывают раз в кадр, своего цикла не
имеет и аллоцирует каждый вызов; тогда цикл надо держать снаружи или брать
постоянный буфер. Арена внутри вложенного цикла освобождается после внутреннего,
то есть переиспользуется в пределах внутреннего цикла.

## 2026-11-23 — `par` renamed to `go`

**Контекст:** `par { }`, `par for` and `par f()` made one keyword carry three
shapes, and the user wanted one word for concurrency.
**Решение:** `par { }` → `go { }`, `par for` → `go for`, `par f(...)` →
detached `go f(...)`. `par` is rejected with a diagnostic pointing at `go`.
**Почему:** `go` reads as "start this", and the shape (block, loop, call)
disambiguates without a second keyword.
**Что отвергли:** `spawn`, `queue`, a separate `parallel for`.
**Цена:** every `par` snippet and diagnostic code renamed once (`par-*` →
`go-*`).

## 2026-11-23 — `within` and hidden context removed; explicit `ctx`

**Контекст:** hidden context coloured functions across the FFI boundary, reset
at package and function-value boundaries, and forced `_ctx` wrappers. `within`
only cancelled calls that observed the context anyway.
**Решение:** there is no hidden context. `context.Context` is an ordinary value
passed explicitly; `within D` becomes
`var ctx, cancel = context.WithTimeout(parent, D); defer cancel();`. `go` does
not create, cancel or derive a context, and cancelling one does not stop
workers.
**Почему:** the user chose option 1: it is visible where `ctx` came from, with
no colouring and no wrapper functions.
**Что отвергли:** hidden context with a `Background()` fallback; passing `ctx`
through every function by default.
**Цена:** functions that need a deadline take `ctx` explicitly; the price is
one parameter on the facade methods that pass it to Go. A deadline no longer
cancels workers.

## 2026-11-23 — Errors are `on err` and `or fail`

**Контекст:** `or err { }` overloaded `or` with handling, `on fail` was the
scope-handler candidate, and `or panic` mixed panic into the error channel.
**Решение:** `on err { }` handles: inline after an expression, or as the first
statement of a function/block as a scope listener for unhandled `go` errors.
`or fail` / `or fail("ctx")` propagate; `fail(...)` stays inside a handler.
`or err`, `on fail` and `or panic` are rejected. The listener cannot be empty
or use `fail`/`return`. A fallible call as a bare statement is rejected;
`_ = f();` is the explicit discard.
**Почему:** two words, one direction each; the user asked for maximal
unification and a listener attached to a scope, not to the program.
**Что отвергли:** `on fail` for propagation, `or panic`, a global handler.
**Цена:** `or err` snippets were rewritten; the listener is captured at launch
and may run after its scope returns.

## 2026-11-23 — Panic is Go's panic, not an error

**Контекст:** `or panic` and the worker recovery boundary turned a panic into
`PanicError`, cancelled siblings and delivered it as an error. The user said
panic must fly where Go sends it.
**Решение:** `panic(...)` is a Go panic. `on err` never receives it, `go` does
not recover or wrap it, and there is no `PanicError` or `AsError`. A panic in
any goroutine prints the stack and exits the process with status 2.
**Почему:** one failure channel per mechanism; wrapping hid the stack and gave
`void` functions recovery behaviour Go does not have.
**Что отвергли:** `or panic`, worker recovery, `PanicError`.
**Цена:** a panic in a worker kills the process; cleanup must use `defer`, not
`on err`.

## 2026-11-23 — Detached `go f(args)`

**Контекст:** `par` only joined, so a background job (metrics, save, work that
outlives a request) had no Ink form and needed a Go adapter.
**Решение:** `go f(args);` starts `f` as a detached task. `f` returns `void` or
`void!`; arguments are evaluated before the task starts; there is no closure.
The task is owned by the program. An unhandled error goes to the scope's
`on err` captured at launch, otherwise it crashes the program. `or fail` after
the call is rejected.
**Почему:** one keyword covers join and detach; the user wanted the whole
project in Ink without a Go adapter for background work.
**Что отвергли:** pools, queues, handles and `send`.
**Цена:** state travels through arguments only; there is no `wait`, so shutdown
and coordination stay in a Go binding.

## 2026-11-23 — Struct contract blocks instead of Go tags

**Контекст:** JSON APIs need external field names and subsets; Go tags are
untyped strings with no compiler check, and the owner rejected copying that
anti-pattern into Ink.
**Решение:** a struct body may declare blocks. `wire { Field: "name"; };` is the
default view and compiles to Go `json` tags; any other lowercase block name is a
named view and compiles to a package-local method returning `map[str, any]`.
A block is an allowlist; named views must be used. Blocks live only inside the
type body; there are no outside blocks.
**Почему:** one declaration owns external names, unlisted fields cannot leak,
and the compiler checks fields, duplicates and names. The owner chose this over
field tags and over one big `contracts` block.
**Что отвергли:** Go struct tags, `wire Admin`, `check Admin`, named
`store`/`doc`/`policy`, top-level blocks, generated view types.
**Цена:** named views are package-local and encode-only; generic structs are
rejected; map views lose declaration order.

## 2026-11-23 — `fail` in a scope listener escalates

**Контекст:** a scope listener could only log, record or `panic`; `fail` was
rejected, so a handler that wanted to escalate had to swallow its own errors or
panic. The generated closure also had no return value, and an `or fail` inside a
listener reached `go build` as "too many return values".
**Решение:** the listener closure returns `error`; `fail(err, "ctx")` inside it
wraps and returns to the enclosing function. `return` stays rejected. In a
detached `go f(...)` task there is no caller to return to, so a failing listener
panics.
**Почему:** the owner reads `fail` as "pass the error up", which is what a
listener needs; the nearest lexical listener already owns escalation.
**Что отвергли:** keeping the ban, panicking instead of escalating, adding a
listener handle.
**Цена:** the generated listener closure has a return value; a detached task's
listener can only panic.

## 2026-11-23 — `errors.New` is the only error constructor

**Контекст:** `error("msg")` compiled to `errors.New("msg")` and duplicated the
Go constructor under the name of the type. `error` is also a type, so
`error(x)` read as a conversion that does not exist.
**Решение:** `error("msg")` is rejected with `error-ctor` and the hint
`write errors.New("msg")`; `errors.New` is the canonical constructor. The code
generator keeps emitting `errors.New` for the rejected form so the diagnostic is
the only error.
**Почему:** one way per intent; the owner picked `errors.New` when asked which
of the two to keep.
**Что отвергли:** keeping both, removing `errors.New` in favour of a builtin.
**Цена:** every error needs `use errors;`; the short form is gone from the tour.

## 2026-11-23 — A detached task owns its errors

**Контекст:** `go f(args)` delivered errors to the `on err` listener of the scope
where the call was written, captured at launch, and crashed the program without
one. The owner rejected both the crash and a program-root listener: a background
`go longsort()` must not put `main` under a panic threat. Making `void!` illegal
in `go f()` was rejected as the start of coloured concurrency.
**Решение:** the call takes its own handler: `go f(args) on err { ... };`. A
`void!` call without one is rejected. The handler runs in the task, sees only
`err` and package-level names, and cannot `fail` or `return`. The scope listener
never receives a detached task error. Any signature is accepted, so `go` stays
colour-blind.
**Почему:** this is the Go pattern — `go func(){ if err := f(); err != nil { log } }()`
— expressed with the handler the language already has, and it keeps the
"no silently dropped error" rule without a runtime root slot.
**Что отвергли:** panic as the only path, a program-root listener, a `void`-only
rule, silent discard, `errgroup`-like handles.
**Цена:** a background failure can only be logged or escalated from inside the
task; supervision stays in a Go adapter.

## 2026-11-23 — Exit does not wait for detached tasks

**Контекст:** a detached `go f(args)` started right before `main` returns is
dropped. Joining program-owned tasks at exit would fix that, but changes exit
semantics for every Ink program.
**Решение:** no join at exit. The process ends when `main` returns, exactly like
Go: goroutines and their `defer`s do not run. `go { ... }` or an explicit
`sync.WaitGroup` is the way to guarantee completion.
**Почему:** the owner asked for Go's behaviour where Go has one; adding a
runtime wait would be a language rule Go does not have.
**Что отвергли:** a program wait group at exit, a root-scope join.
**Цена:** fire-and-forget work can be lost at exit; this is documented in EDGES.

## 2026-11-23 — Blind-test round: per-test failures, docs for what agents missed

**Контекст:** five parallel blind agents (HTTP service, raylib Snake, planted
data-layer bugs, SQLite ledger, Go port) scored 6-7/10. All five lost time on
the same undocumented essentials: the `test "name" { ... };` form (three wrote
`Test*` functions, which silently compile), `str(65)` being `"A"`, block
parentheses and `};`, `int` vs `i32`, import aliases and `use pkg as _;`, and
`GOFLAGS=-tags=nocgo` for raylib. One `assert_eq` panic killed the test binary,
so only the first of four failing tests was visible.
**Решение:** `ink guide` and the generated project guide now document tests,
control flow, literals, conversions, imports/dependencies, reserved words and
the deliberately absent set. The generated test wrapper recovers panics into
`t.Fatalf`. `ink test` explains an empty suite. `ink run` forwards
SIGINT/SIGTERM. Unused-import and `use _ pkg;` hints name the fix, and `error`
is nameable, so `ctx.Err() != nil` checks work. The tasks live in `tests/blind/`.
**Почему:** the agent is the main consumer; a silent wrong result costs more
than a longer guide, and a panic should not hide the remaining failures.
**Что отвергли:** changing `str(int)` to decimal, warning on it, rejecting a
fallible `defer`, adding channels. All are language decisions left open.
**Цена:** the guide grew by about a third; recovering a panic loses its stack
trace; a terminal Ctrl-C can reach the child twice, once from the group and
once forwarded.

## 2026-11-24 — `//go:embed` — флаг сборки, а не язык

**Контекст:** EDGES #3. В языке нет `//go:embed`, поэтому шаблоны и статика
живут рядом с бинарём. Варианты из `docs/proposals/edges.md`: атрибут в
исходнике (3b), встроенная форма `assets("dir")` (3a), флаг `ink build` (3d).
**Решение:** embed — забота сборки, не синтаксиса. Ни атрибута, ни встроенной
формы в языке не будет; переключатель — во флагах `ink build`.
**Почему:** всё происходит только на сборке, и владелец сказал, что тащить это
в язык избыточно: «место галочкам для сборки в флаги для инк билд».
**Что отвергли:** `//go:embed` как атрибут (открывает attributes вне §10),
builtin `assets("dir")` (лишняя поверхность языка).
**Цена:** до появления флага статика читается с диска или через Go-биндинг с
`embed.FS`; форма доступа к встроенному FS определится при реализации флага.

## 2026-11-24 — полный `ink check` гоняет `go vet`

**Контекст:** `str(42)` — это Go-шная конверсия в руну; `ink check` был чист,
а `ink test` падал на `go vet`. Запись от 2026-11-23 отвергла предупреждение
в sema, потому что `rune` и число неразличимы по типу.
**Решение:** полный `ink check` запускает `go vet` через `-overlay` и маппит
находки в `.ink`-позиции; `string of one rune` получает код `str-number` с
hint на `strconv`/`fmt`. `--fast` и LSP vet не гоняют.
**Почему:** vet различает `rune` и число сам (`string(rune)` он не трогает),
а гейт `check` обязан ловить то, на чём падает следующая команда.
**Что отвергли:** предупреждение в sema (не различает rune), запрет `str` на
числах (ломает rune-конверсию), писать сгенерированные файлы ради vet.
**Цена:** полный check на проект дороже на один `go vet`; overlay пишется в
временный каталог, проект остаётся read-only.

## 2026-11-24 — `make` удалён, срез конструируется своим типом

**Контекст:** `make` существовал только ради длины и ёмкости slice; `var`
закрывает map (решение 12a на паузе), литерал, структуру и указатель. Владелец
просил убрать служебное слово из инициализации и выбрать читаемую форму.
**Решение:** `T[](n)` — n нулей, `T[](n, cap: m)` — размер и запас,
`T[](cap: m)` — пустой с запасом, `T[](size: n)` — то же, что `T[](n)`.
Аргументы именованные; `size` опускается, `cap` — метка. `make` в исходнике
даёт `make-removed` с hint. `T[]()` внутри `arena` берёт память из арены.
**Почему:** тип — уже существующий конструктор (`i32(x)`, `[]byte(s)`), и
`var buf = u8[](cap: 4096)` читается как инициализация без служебного слова;
именованные аргументы снимают вопрос «что здесь 0, а что 4096».
**Что отвергли:** `slices.Grow` (лишний Go-вызов вместо записи), `zeros[u8](n)`
(слово остаётся), `u8[] of n` (ключевое слово ради одного случая), позиционные
`(n, cap)` (непонятно без подписей).
**Цена:** `make` исчез из языка; элемент среза должен быть именуемым — срез
указателей требует алиаса. `cap` без `size` синтезирует длину 0.

## 2026-11-24 — map не выделяется неявно; `= nil` остаётся явным

**Контекст:** EDGES #12: запись в nil-map паникует, `ink check` молчит. Варианты:
`var m: map[K,V]` выделяет map (12a), warning на запись (12b), `make(map)` (12c).
**Решение:** 12b. Неявной инициализации нет: `var` всегда требует `= value`,
поэтому `var m: map[K,V] = nil` — явный nil, а `map[K,V] {}` — конструктор
рабочей map. Запись по индексу в nil-map даёт warning `nil-map-write`.
**Почему:** формы `var m: map[K,V]` без `=` в языке нет, и владелец не хочет
двоеточия в создании; `= nil` и литерал уже выражают оба намерения явно.
**Что отвергли:** 12a (неявное выделение, меняет `m == nil`), 12c
(`make(map[K,V])` — слово, которое мы только что убрали).
**Цена:** поле структуры с map-типом остаётся nil до присваивания; warning
видит только переменные с явным `= nil`, не поля и не параметры.
