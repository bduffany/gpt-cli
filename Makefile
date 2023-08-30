SRCS := $(shell find . -name "*.go")

all: gpt

go.sum: go.mod
	go mod tidy

gpt: $(SRCS) internal/auto/auto.md go.sum
	go build -o . ./...
