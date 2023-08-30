# gpt-cli

Fast, simple, powerful GPT CLI client written in Go.

- Uses the streaming API for realtime output.
- Keeps chat context throughout the session.
- Supports reading input from stdin, for integration in command pipelines.

## Install

```
go install github.com/bduffany/gpt-cli/cmd/gpt@latest
```

## Usage

Configure API key:

```shell
echo >> ~/.bashrc 'export OPENAI_API_KEY=YOUR_API_KEY'
exec bash
```

Run an interactive session, with context preserved across prompts:

```shell
$ gpt
you> You're a sorter. I give you a list, you reply only with the sorted list.
Understood, please provide the list you would like sorted.
you> b, c, a
a, b, c
```

Pipe prompts to stdin:

````shell
$ echo >prompt.txt 'Generate TS definitions from Go structs. ONLY output code. Omit ```:'
$ echo >api.go 'type Foo struct { Bar string `json:"bar"` }'
$ cat prompt.txt api.go | gpt | tee api.ts
interface Foo {
  bar: string;
}
````
