#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
root=$PWD
ink=$root/bin/ink
if [ ! -x "$ink" ]; then
  echo "tests/run.sh: build bin/ink first" >&2
  exit 1
fi
fail=0
cleanup() {
  local d="$1"
  rm -rf "$d/.ink" "$d/bin" 2>/dev/null || true
  find "$d" -name 'zz_ink.go' -o -name 'zz_ink_test.go' 2>/dev/null | while read -r f; do rm -f "$f"; done
  rm -f "$d/go.mod" "$d/go.sum" 2>/dev/null || true
}
# shortcut: flat JSON without nested objects or escaped quotes, replace sed parsing if the contract grows.
json_field() {
  printf '%s' "$1" | sed -n "s/.*\"$2\":\"\\([^\"]*\\)\".*/\\1/p"
}
json_number() {
  printf '%s' "$1" | sed -n "s/.*\"$2\":\\([0-9][0-9]*\\).*/\\1/p"
}
check_tour() {
  local src="$root/examples/tour" dir out err
  dir=$(mktemp -d)
  out=$(mktemp)
  err=$(mktemp)
  cp -R "$src/." "$dir/"
  cleanup "$dir"
  if ! (cd "$dir" && timeout 120 "$ink" check >"$out" 2>"$err"); then
    echo "FAIL examples/tour (check)" >&2
    cat "$err" >&2
    fail=1
  fi
  if ! (cd "$dir" && timeout 180 "$ink" test >"$out" 2>"$err"); then
    echo "FAIL examples/tour (test)" >&2
    cat "$err" >&2
    fail=1
  fi
  if ! (cd "$dir" && timeout 120 "$ink" run >"$out" 2>"$err"); then
    echo "FAIL examples/tour (run)" >&2
    cat "$err" >&2
    fail=1
  elif ! diff -u "$dir/expected.txt" "$out"; then
    echo "FAIL examples/tour (output)" >&2
    fail=1
  fi
  rm -rf "$dir"
  rm -f "$out" "$err"
}
check_problems() {
  local dir="$root/examples/problems" out err objects status total actual i
  local obj line col code msg want wline wcol wcode wmsg rest got wgot
  out=$(mktemp)
  err=$(mktemp)
  objects=$(mktemp)
  if (cd "$dir" && timeout 120 "$ink" check --json >"$out" 2>"$err"); then
    status=0
  else
    status=$?
  fi
  if [ "$status" -ne 1 ]; then
    echo "FAIL examples/problems (check exit $status, want 1)" >&2
    cat "$out" "$err" >&2
    fail=1
    rm -f "$out" "$err" "$objects"
    return
  fi
  tr -d '\n' <"$out" | sed 's/},{/}\n{/g' >"$objects"
  printf '\n' >>"$objects"
  total=$(wc -l <"$dir/expected.txt")
  actual=$(wc -l <"$objects")
  if [ "$actual" -ne "$total" ]; then
    echo "FAIL examples/problems (want $total diagnostics, got $actual)" >&2
    cat "$objects" >&2
    fail=1
    rm -f "$out" "$err" "$objects"
    return
  fi
  i=0
  while IFS= read -r want; do
    i=$((i + 1))
    obj=$(sed -n "${i}p" "$objects")
    line=$(json_number "$obj" line)
    col=$(json_number "$obj" col)
    code=$(json_field "$obj" code)
    msg=$(json_field "$obj" message)
    wline=${want%%:*}
    rest=${want#*:}
    wcol=${rest%%:*}
    rest=${rest#*:}
    wcode=${rest%%:*}
    wmsg=${rest#*:}
    got="$line:$col:$code"
    wgot="$wline:$wcol:$wcode"
    if [ "$got" != "$wgot" ]; then
      echo "FAIL examples/problems (diagnostic $i): want $wgot, got $got" >&2
      fail=1
      continue
    fi
    case "$msg" in
      "$wmsg"*) ;;
      *)
        echo "FAIL examples/problems (diagnostic $i message): want prefix '$wmsg', got '$msg'" >&2
        fail=1
        ;;
    esac
  done <"$dir/expected.txt"
  rm -f "$out" "$err" "$objects"
}
for dir in "$root"/tests/cases/*/; do
  name=$(basename "$dir")
  out=$(mktemp)
  err=$(mktemp)
  if [ -f "$dir/expected-fast.txt" ] || [ -f "$dir/expected-check.txt" ]; then
    objects=$(mktemp)
    expected="$dir/expected-fast.txt"
    check_args=(--json --fast)
    if [ -f "$dir/expected-check.txt" ]; then
      expected="$dir/expected-check.txt"
      check_args=(--json)
    fi
    if (cd "$dir" && timeout 120 "$ink" check "${check_args[@]}" >"$out" 2>"$err"); then
      echo "FAIL $name (check exited 0, want 1)" >&2
      fail=1
    else
      check_status=$?
      if [ "$check_status" -ne 1 ]; then
        echo "FAIL $name (check exited $check_status, want 1)" >&2
        cat "$out" "$err" >&2
        fail=1
      else
        tr -d '\n' <"$out" | sed 's/},{/}\n{/g' >"$objects"
        printf '\n' >>"$objects"
        total=$(wc -l <"$expected")
        actual=$(wc -l <"$objects")
        if [ "$actual" -ne "$total" ]; then
          echo "FAIL $name (want $total diagnostics, got $actual)" >&2
          cat "$objects" >&2
          fail=1
        else
          i=0
          while IFS= read -r want; do
            i=$((i + 1))
            obj=$(sed -n "${i}p" "$objects")
            line=$(json_number "$obj" line)
            col=$(json_number "$obj" col)
            code=$(json_field "$obj" code)
            msg=$(json_field "$obj" message)
            wline=${want%%:*}
            rest=${want#*:}
            wcol=${rest%%:*}
            rest=${rest#*:}
            wcode=${rest%%:*}
            wmsg=${rest#*:}
            got="$line:$col:$code"
            wgot="$wline:$wcol:$wcode"
            if [ "$got" != "$wgot" ]; then
              echo "FAIL $name (diagnostic $i): want $wgot, got $got" >&2
              fail=1
            fi
            case "$msg" in
              "$wmsg"*) ;;
              *)
                echo "FAIL $name (diagnostic $i message): want prefix '$wmsg', got '$msg'" >&2
                fail=1
                ;;
            esac
          done <"$expected"
        fi
      fi
    fi
    rm -f "$objects"
    rm -f "$out" "$err"
    cleanup "$dir"
    continue
  fi
  if [ -f "$dir/test" ]; then
    if ! (cd "$dir" && timeout 180 "$ink" test -v >"$out" 2>"$err"); then
      echo "FAIL $name (test)" >&2
      cat "$err" >&2
      fail=1
      rm -f "$out" "$err"
      cleanup "$dir"
      continue
    fi
    declared=$(grep -Rh '^test "' --include='*.ink' "$dir" | wc -l || true)
    declared=${declared//[[:space:]]/}
    passed=$(grep -c '^--- PASS:' "$out" || true)
    passed=${passed//[[:space:]]/}
    if grep -q '\[no test files\]' "$out"; then
      echo "FAIL $name (no test files)" >&2
      fail=1
    elif [ "$declared" -eq 0 ] || [ "$passed" -ne "$declared" ]; then
      echo "FAIL $name (declared $declared, passed $passed)" >&2
      cat "$out" >&2
      fail=1
    else
      echo "$name: $declared declared, $passed passed"
    fi
    if [ -f "$dir/expected-test.txt" ] && ! diff -u "$dir/expected-test.txt" <(grep -E '^(ok|FAIL|---)' "$out" || true); then
      fail=1
    fi
    rm -f "$out" "$err"
    cleanup "$dir"
    continue
  fi
  if ! (cd "$dir" && timeout 120 "$ink" run >"$out" 2>"$err"); then
    echo "FAIL $name (run)" >&2
    cat "$err" >&2
    fail=1
    rm -f "$out" "$err"
    cleanup "$dir"
    continue
  fi
  if [ -f "$dir/expected.txt" ]; then
    if ! diff -u "$dir/expected.txt" "$out"; then
      echo "FAIL $name (output)" >&2
      fail=1
    fi
  fi
  rm -f "$out" "$err"
  cleanup "$dir"
done
check_tour
check_problems
if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "tests: ok"
