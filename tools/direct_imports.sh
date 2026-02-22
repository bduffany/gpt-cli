#!/usr/bin/env sh
set -eu

module_path="${1:-}"
if [ -z "$module_path" ]; then
  echo "module path required" >&2
  exit 1
fi

go list -f '{{join .Imports "\n"}}' ./... | sort -u | \
while IFS= read -r imp; do
  if [ -z "$imp" ]; then
    continue
  fi
  go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}' "$imp"
done | \
awk -v mod="$module_path" 'NF && $0 != mod && index($0, mod "/") != 1 { print $0 }' | \
sort -u
