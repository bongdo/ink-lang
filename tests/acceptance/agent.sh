#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
ink=$root/bin/ink
if [ ! -x "$ink" ]; then
  echo "tests/acceptance/agent.sh: build bin/ink first" >&2
  exit 1
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
die() {
  echo "agent: FAIL: $*" >&2
  exit 1
}
# shortcut: flat JSON without nested objects or escaped quotes, replace sed parsing if the contract grows.
json_field() {
  printf '%s' "$1" | sed -n "s/.*\"$2\":\"\\([^\"]*\\)\".*/\\1/p"
}
json_number() {
  printf '%s' "$1" | sed -n "s/.*\"$2\":\\([0-9][0-9]*\\).*/\\1/p"
}
write_program() {
  cat > main.ink <<'EOF'
use fmt;
use errors;

const Answer = 42;

struct Counter {
    n: i32,

    fn Add(d: i32) void {
        n += d;
    };

    fn Value() i32 {
        return n;
    };
};

fn twice(n: i32) i32! {
    if (n < 0) {
        return errors.New("negative");
    };
    return n * 2;
};

test "counter" {
    var c = Counter { n: 1 };
    c.Add(2);
    assert_eq(c.Value(), 3);
};

fn main() void! {
    var c = Counter { n: 1 };
    c.Add(Answer - 39);
    var v = twice(c.Value()) or fail;
    fmt.Println("value", v) or fail;
    return;
};
EOF
}
"$ink" new "$tmp/hello" >/dev/null || die "ink new failed"
cd "$tmp/hello"
write_program
if ! out=$("$ink" check --json --fast); then
  die "check --json --fast failed"
fi
[ "$out" = "[]" ] || die "check --json --fast: want [], got '$out'"
awk '{ if ($0 == "    c.Add(Answer - 39);") print "    const N = 3;"; print }' main.ink > main.tmp
mv main.tmp main.ink
if "$ink" check --json >diag.json 2>diag.err; then
  die "check exited 0 for a local const"
else
  status=$?
fi
[ "$status" -eq 1 ] || die "local const: check exited $status, want 1"
grep -q '"code":"const-local"' diag.json || die "local const: const-local missing: $(cat diag.json)"
tr -d '\n' <diag.json | sed 's/},{/}\n{/g' >objects.txt
printf '\n' >>objects.txt
obj=$(sed -n '/"code":"const-local"/{p;q;}' objects.txt)
[ -n "$obj" ] || die "local const: diagnostic object not found"
file=$(json_field "$obj" file)
line=$(json_number "$obj" line)
col=$(json_number "$obj" col)
end_line=$(json_number "$obj" endLine)
end_col=$(json_number "$obj" endCol)
[ "$file" = "$PWD/main.ink" ] || die "const-local file: want '$PWD/main.ink', got '$file'"
for n in "$line" "$col" "$end_line" "$end_col"; do
  case "$n" in
    ''|*[!0-9]*) die "const-local: bad position field '$n'" ;;
  esac
  [ "$n" -gt 0 ] || die "const-local: position field is zero"
done
write_program
awk '{ if ($0 == "    var v = twice(c.Value()) or fail;") { print "    var v, err = twice(c.Value());"; print "    if (err != nil) {"; print "        return err;"; print "    };"; next } print }' main.ink > main.tmp
mv main.tmp main.ink
if "$ink" check --json >diag.json 2>diag.err; then
  die "check exited 0 for manual error propagation"
else
  status=$?
fi
[ "$status" -eq 1 ] || die "manual propagation: check exited $status, want 1"
grep -q '"code":"err-manual"' diag.json || die "manual propagation: err-manual missing: $(cat diag.json)"
tr -d '\n' <diag.json | sed 's/},{/}\n{/g' >objects.txt
printf '\n' >>objects.txt
obj=$(sed -n '/"code":"err-manual"/{p;q;}' objects.txt)
[ -n "$obj" ] || die "manual propagation: diagnostic object not found"
hint=$(json_field "$obj" hint)
[ -n "$hint" ] || die "manual propagation: hint is empty"
write_program
sed 's/    var c = Counter { n: 1 };/    var  c = Counter { n = 1 };/' main.ink > main.tmp
mv main.tmp main.ink
listed=$("$ink" fmt -l --json) || die "fmt -l --json failed"
[ "$listed" = '["main.ink"]' ] || die "fmt -l --json: want [\"main.ink\"], got '$listed'"
"$ink" fmt -w . || die "fmt -w . failed"
listed=$("$ink" fmt -l --json) || die "fmt -l --json after write failed"
[ "$listed" = "[]" ] || die "fmt -l --json after write: want [], got '$listed'"
if ! "$ink" test -v >test.out 2>&1; then
  cat test.out >&2
  die "ink test failed"
fi
passed=$(grep -c '^--- PASS:' test.out || true)
[ "$passed" -eq 1 ] || die "ink test: want 1 PASS, got $passed"
grep -q '^--- PASS: TestCounter' test.out || die "ink test: TestCounter did not run"
if grep -q '\[no test files\]' test.out; then
  die "ink test: no test files"
fi
if ! run_out=$("$ink" run 2>run.err); then
  cat run.err >&2
  die "ink run failed"
fi
[ "$run_out" = "value 8" ] || die "ink run: want 'value 8', got '$run_out'"
rm -rf .ink go.mod go.sum zz_ink.go zz_ink_test.go
for f in .ink go.mod go.sum zz_ink.go zz_ink_test.go; do
  [ ! -e "$f" ] || die "cleanup left $f"
done
echo "agent: ok"
