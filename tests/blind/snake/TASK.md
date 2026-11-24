# Blind task: Snake game with raylib

You are testing Ink as a language for an AI agent, blind: you have never used
Ink before and must learn it only from `ink guide`.

## Setup

This directory is a template. Create a fresh project outside the Ink
repository and work only there, for example:

```sh
cd /tmp && /home/ubuntu/Projects/Ink/bin/ink new blind-snake && cd blind-snake
cp /path/to/tests/blind/snake/TASK.md .
```

## Hard rules

- Work only in the copied project directory.
- `ink` is not on PATH. Use `/home/ubuntu/Projects/Ink/bin/ink` (call it `INK`).
- Do **not** read, list or grep anything under `/home/ubuntu/Projects/Ink`
  (compiler source, examples, docs). The only allowed reference is
  `$INK guide` output and `$INK --help`.
- Do not edit files under `/home/ubuntu/Projects/Ink`. Do not install system
  packages. Do not use the `explore` agent or web search.
- Record friction **while working**, not from memory at the end.

## Environment facts

- There is no display, no `libraylib`, and no X11 headers on this machine.
  `ink run` of a raylib program cannot work here. Compile-level verification is
  the acceptance gate; say so in the report instead of faking a screenshot.
- `github.com/gen2brain/raylib-go/raylib` is in the module cache. Module
  downloads work.
- You must discover how to make `ink check` build raylib on this machine
  (there is a documented-enough way; do not guess it from the compiler repo).

## Task

Build a playable Snake game in Ink with raylib:

- 20x20 grid, window 600x600, cells drawn with `DrawRectangle`, snake head
  distinguished by color, food as a circle or square.
- Movement on a tick timer (e.g. 8 moves/second), not one cell per frame.
- Arrow keys and WASD; direct 180-degree reversal ignored.
- Eating food grows the snake, increments score, respawns food on a free cell.
- Wall collision and self collision end the game.
- Game over screen shows score and "press R to restart"; R restarts cleanly.
- Score drawn on screen while playing.
- Frame-rate independent timing based on `GetFrameTime` or `GetTime`.
- Pause on Space.

Keep the game rules in pure functions/small structs separate from rendering:
`step(state, dir) State`, `spawnFood(state, seed) Position`,
`collides(state) bool`, etc., so they are testable without a window.

Acceptance:

1. `$INK check --json` prints `[]` (including the raylib import; find the build
   tags/GOFLAGS that make it compile here).
2. `$INK test` green with tests for: growth on food, food never spawns on the
   snake, wall collision, self collision, reversal ignored, restart resets
   score/length. Pure logic only, no window.
3. `$INK fmt -l --json` prints `[]`.
4. State explicitly what could not be verified without a display.

## REPORT.md (required, in this directory)

```md
# Snake — blind report

## Result
What compiles and what is tested; exact commands and outputs.

## Cycles
- check runs before first `[]`:
- test runs before green:
- total tool calls (approx):
- wall time:

## Friction log
| # | what I tried | exact error / surprise (quote the message) | cost | what would have fixed it |
|---|---|---|---|---|

## Guide gaps
Exact sentences missing from `ink guide`, ready to paste.

## Suspected Ink bugs
Minimal repro per suspicion, or "none".

## Verdict
Score 1-10 for agent-friendliness of this task, biggest time sink.
```

Budget: under 45 minutes and 60 tool calls. If blocked after 3 honest attempts,
stop that part and report it as a gap; never invent syntax.
