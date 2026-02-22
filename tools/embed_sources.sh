#!/usr/bin/env sh
set -eu

root="${1:-.}"

(rg -n --no-heading --glob '*.go' 'go:embed' "$root" || true) | \
awk -F: '{
  file=$1
  line=$0
  sub(/^[^:]*:[0-9]+:/,"",line)
  sub(/.*go:embed[[:space:]]+/,"",line)
  n=split(line,parts,/[[:space:]]+/)
  dir="."
  if (index(file, "/") > 0) { dir=file; sub(/\/[^\/]+$$/,"",dir) }
  for (i=1; i<=n; i++) {
    if (parts[i] != "") { print dir "/" parts[i] }
  }
}'
