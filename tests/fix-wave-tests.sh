#!/bin/sh

set -u

script_dir=$(CDPATH='' cd "$(dirname "$0")" && pwd)
repo_root=$(CDPATH='' cd "$script_dir/.." && pwd)
fixture_dir=$(mktemp -d "${TMPDIR:-/tmp}/tgo-fix-wave.XXXXXX") || exit 1
failures=0

cleanup() {
    status=$?
    trap - EXIT HUP INT TERM
    rm -rf -- "$fixture_dir"
    if [ "$failures" -ne 0 ] && [ "$status" -eq 0 ]; then
        status=1
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    failures=$((failures + 1))
}

run_capture() {
    output_file=$1
    shift
    if "$@" >"$output_file" 2>&1; then
        command_status=0
    else
        command_status=$?
    fi
}

mode_of() {
    if [ "$(uname -s)" = Darwin ]; then
        stat -f '%Lp' "$1"
    else
        stat -c '%a' "$1"
    fi
}

assert_no_temp_files() {
    directory=$1
    pattern=$2
    if find "$directory" -type f -name "$pattern" -print | grep -q .; then
        fail "temporary files remain for $pattern"
    fi
}

fake_bin=$fixture_dir/bin
mkdir "$fake_bin"

cat > "$fake_bin/hk" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" > "$HK_MARKER"
exit 0
EOF
chmod 700 "$fake_bin/hk"
HK_MARKER=$fixture_dir/hk-called
export HK_MARKER
multiline_title=$(printf 'feat: add export\nfix: second')
run_capture "$fixture_dir/multiline.out" env PATH="$fake_bin:$PATH" \
    "$repo_root/.mise/tasks/pr-title" "$multiline_title"
if [ "$command_status" -eq 0 ]; then
    fail 'multiline PR title was accepted'
fi
if grep -Fq 'invalid PR title' "$fixture_dir/multiline.out"; then
    :
else
    fail 'multiline PR title did not produce the concise invalid-title diagnostic'
fi
if [ -e "$HK_MARKER" ]; then
    fail 'multiline PR title invoked hk instead of being rejected as one title'
fi

cat > "$fake_bin/hk" <<'EOF'
#!/bin/sh
printf '%s\n' 'simulated hk config failure' >&2
exit 2
EOF
chmod 700 "$fake_bin/hk"
rm -f -- "$HK_MARKER"
run_capture "$fixture_dir/hk-failure.out" env PATH="$fake_bin:$PATH" \
    "$repo_root/.mise/tasks/pr-title" 'feat: add export'
if [ "$command_status" -ne 2 ]; then
    fail "hk failure was remapped to status $command_status instead of 2"
fi
if grep -Fq 'simulated hk config failure' "$fixture_dir/hk-failure.out"; then
    :
else
    fail 'hk failure output was not surfaced'
fi
if grep -Fq 'invalid PR title' "$fixture_dir/hk-failure.out"; then
    fail 'hk command failure was masked as an invalid title'
fi

mixed_file=$fixture_dir/mixed.txt
printf 'one\ntwo\r\nthree\n' > "$mixed_file"
chmod 640 "$mixed_file"
mixed_mode_before=$(mode_of "$mixed_file")
run_capture "$fixture_dir/mixed.out" mise exec -- hk util mixed-line-ending --fix "$mixed_file"
if [ "$command_status" -ne 0 ]; then
    fail 'hk mixed-line-ending builtin did not fix a mixed-ending fixture'
fi
if LC_ALL=C grep -q "$(printf '\r')" "$mixed_file"; then
    fail 'hk mixed-line-ending builtin left CR bytes in the fixture'
fi
if [ "$(mode_of "$mixed_file")" != "$mixed_mode_before" ]; then
    fail 'hk mixed-line-ending builtin did not preserve the source mode'
fi

trailing_file=$fixture_dir/notes.md
printf 'hard break   \nplain \t\n' > "$trailing_file"
chmod 640 "$trailing_file"
trailing_mode_before=$(mode_of "$trailing_file")
run_capture "$fixture_dir/trailing.out" mise exec -- hk util trailing-whitespace --fix "$trailing_file"
if [ "$command_status" -ne 0 ]; then
    fail 'hk trailing-whitespace builtin did not fix a Markdown fixture'
fi
if ! printf 'hard break\nplain\n' | cmp -s - "$trailing_file"; then
    fail 'hk trailing-whitespace builtin did not trim all trailing whitespace'
fi
if [ "$(mode_of "$trailing_file")" != "$trailing_mode_before" ]; then
    fail 'hk trailing-whitespace builtin did not preserve the source mode'
fi


if command -v mise >/dev/null 2>&1 && mise exec -- yamlfmt --version >/dev/null 2>&1; then
    yaml_file=$fixture_dir/crlf.yaml
    printf '%s\r\n' '---' 'key: value' > "$yaml_file"
    if (cd "$repo_root" && mise exec -- yamlfmt "$yaml_file") >"$fixture_dir/yamlfmt.out" 2>&1; then
        if LC_ALL=C grep -q "$(printf '\r')" "$yaml_file"; then
            fail 'yamlfmt reintroduced CRLF in the CRLF fixture'
        fi
    else
        fail 'yamlfmt could not format the CRLF fixture'
    fi
fi

if [ "$failures" -eq 0 ]; then
    printf '%s\n' 'fix-wave focused fixtures passed'
else
    printf '%s focused fixture(s) failed\n' "$failures" >&2
fi
exit "$failures"
