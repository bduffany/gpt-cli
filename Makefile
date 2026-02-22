GO_SRCS := $(shell find . -name "*.go")
EMBED_SRCS := $(shell tools/embed_sources.sh .)
IMPORTS_FILE := .go-imported-packages
MODULE_PATH := $(shell go list -m -f '{{.Path}}')

all: gpt

imports: go.sum $(IMPORTS_FILE)
.PHONY: imports

test: go.sum
	go test ./...

tidy-check: SKIP_TIDY=1
tidy-check: go.sum
	@tools/tidy_check.sh

.PHONY: test tidy-check

go.sum: go.mod $(IMPORTS_FILE)
	@if [ "$(SKIP_TIDY)" != "1" ]; then go mod tidy; fi

gpt: go.sum $(GO_SRCS) $(EMBED_SRCS)
	go build -o . ./...

$(IMPORTS_FILE): $(GO_SRCS)
	@tools/direct_imports.sh "$(MODULE_PATH)" > $@.tmp
	@cmp -s $@.tmp $@ 2>/dev/null || mv $@.tmp $@
	@rm -f $@.tmp
