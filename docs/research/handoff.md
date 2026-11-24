# Handoff — три трека

Состояние на 2026-11-24, коммит `b76bfac`. Этот файл — карта для новых чатов:
что уже решено, где лежит правда, какие вопросы открыты.

## Что закрыто

`docs/product/EDGES.md` — раздел Open пуст; всё, что не код, записано в
Accepted с границей и ценой. Решения: `docs/maintain/decisions.md` (последние
записи от 2026-11-24).

- `make` удалён: `T[](n)`, `T[](n, cap: m)`, `T[](cap: m)`, `T[](size: n)`.
- map: `map[K, V] {}` — пустая, `map[K, V](nil)` — nil, неявной
  инициализации нет, запись в nil-map — warning `nil-map-write`.
- полный `ink check` гоняет `go vet`; `str(42)` → `str-number`.
- `ink build -embed dir` + встроенный `asset(path)`; `//go:embed` в языке нет.
- LSP: отмена прохода sema, hover показывает выведенный Go-тип, rename
  отклоняется, если ломает или перепривязывает имя.

## Слепой раунд 2 (запущен)

Три задачи, среда: `/tmp/opencode/blind-{web,bugs,sqlite}`, компилятор
`/home/ubuntu/Projects/Ink/bin/ink`, только `ink guide` и `TASK.md`.
Отчёты появятся в `REPORT.md` внутри этих каталогов.

Прошлый раунд (2026-11-23) — `tests/blind/README.md`, оценки 6–7/10.
С тех пор язык изменился, поэтому раунд 2 нужен заново.

## Трек A — синтаксис

Пишет агенту пользователь. Вход: `ink guide`, `docs/product/SPEC.md`,
`vault/features/syntax.md`, `docs/proposals/`, `docs/maintain/decisions.md`.

Открытые вопросы (не края, а вкус):

- nil-map пишется двумя способами: `map[K, V](nil)` и `nil[map[K, V]]`.
  Оставить один? Диагностика `nil-form` — десять строк.
- Тип элемента среза должен быть именуемым: `*Point[](n)` парсится как
  разыменование, нужен алиас. Документировано, но выглядит как дыра.
- Форматтер переписывает `T[](size: n)` в позиционный `T[](n)`; сохранять
  метку, если она была?
- Внешний код с `make` ломается без автоматической миграции. Нужен ли
  `ink fix`/код-экшен массового переписывания?

## Трек B — опыт разработки для агента

Пишет агенту пользователь. Вход: `opencode.json`, `internal/agent/guide.md`,
`internal/driver`, `internal/lsp`, отчёты слепого раунда.

Факты на сейчас:

- В репозитории компилятора `opencode.json` подключает `bin/ink lsp` и
  `bin/ink fmt -w $FILE`; LSP отвечает на `initialize` (проверено).
- `ink new` пишет `opencode.json` с командой `ink` (из `PATH`) и `.opencode/`
  со skill; если `ink` в `PATH` нет, подставляется абсолютный путь текущего
  бинаря (исправлено 2026-11-24). Осталось решить: предупреждать ли, что
  абсолютный путь делает `opencode.json` непереносимым.
- Глобальный `~/.config/opencode` не содержит LSP для Ink: проект без
  `opencode.json` остаётся без диагностики в редакторе.
- `.gitignore` проектов и этого репозитория уже игнорирует `zz_ink.go`,
  `zz_ink_test.go`, `.ink/` — сгенерированное не попадает в git.
- `ink check` на `examples/tour` — 0.58 s, `--fast` — 0.5 s.

Инструменты (10 команд):

| команда | что делает | аналог в Go |
|---|---|---|
| `ink check` | parse, resolve, sema, typecheck сгенерированного Go, `go vet`; `--fast` без Go; `--json` | `go vet` (+ typecheck) |
| `ink build` | check + генерация + `go build -o bin/<name>`; `-embed dir` | `go build` |
| `ink run` | build во временный бинарь + запуск, `--race`, `-embed`, `-- args` | `go run` |
| `ink test` | check + генерация тестов + `go test ./...` | `go test` |
| `ink fmt` | форматтер; `-l`, `-w`, `--json` | `gofmt` |
| `ink lsp` | LSP на stdio | `gopls` |
| `ink new` / `ink init` | проект: `go.mod`, `main.ink`, `AGENTS.md`, `opencode.json`, skill | `go mod init` + шаблон |
| `ink guide` | справочник из бинаря (он же skill) | — |
| `ink version` | версия | `go version` |

Открытые вопросы:

- `ink check` называть `ink vet`? Или оставить `check` и считать его
  «универсальным vet» (сейчас это ровно он: sema + typecheck + go vet).
- Хранить сгенерированный Go или убирать после сборки: предложение — флаг
  `ink build -temp`, по умолчанию файлы остаются (их видит `go vet`,
  `gopls`, отладчик). Мусор уже не в git.
- Добавить `ink build -temp`: собрать и убрать сгенерированные файлы после
  успешной сборки; по умолчанию файлы остаются.

## Трек C — research и оркестрация (этот чат)

- Гоняет слепые раунды, сводит отчёты в `EDGES.md`/guide.
- Замеряет: время `check`, размер бинаря, стоимость LSP на большом проекте.
- Ведёт перенос реального приложения: вертикальный срез nit-web
  (`/api/health`, `/api/sessions`, `/api/events`) против живого OpenCode.
- Решает, что из отчётов идёт в трек A (синтаксис), что в B (DX), что в код
  компилятора.
