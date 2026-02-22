#!/usr/bin/env sh
set -eu

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT

cp go.mod "$tmpdir/go.mod"
if [ -f go.sum ]; then
  cp go.sum "$tmpdir/go.sum"
else
  rm -f "$tmpdir/go.sum"
fi

if go mod tidy; then
  changed=0
  if ! cmp -s go.mod "$tmpdir/go.mod"; then
    changed=1
  fi
  if [ -f "$tmpdir/go.sum" ]; then
    if ! cmp -s go.sum "$tmpdir/go.sum"; then
      changed=1
    fi
  else
    if [ -f go.sum ]; then
      changed=1
    fi
  fi
  if [ "$changed" -ne 0 ]; then
    cp "$tmpdir/go.mod" go.mod
    if [ -f "$tmpdir/go.sum" ]; then
      cp "$tmpdir/go.sum" go.sum
    else
      rm -f go.sum
    fi
    echo "go.mod/go.sum are not tidy; run 'go mod tidy'"
    exit 1
  fi
else
  status=$?
  cp "$tmpdir/go.mod" go.mod
  if [ -f "$tmpdir/go.sum" ]; then
    cp "$tmpdir/go.sum" go.sum
  else
    rm -f go.sum
  fi
  exit "$status"
fi
