# gpt-cli

Fast, simple, and powerful GPT CLI client written in Go.

- Uses the streaming API for realtime output.
- Keeps chat context throughout the session.
- Supports reading input from stdin, for integration in command pipelines.

## Install

```shell
go install github.com/bduffany/gpt-cli/cmd/gpt@latest
```

## Usage

## Authentication

Generate an [OpenAI API key](https://platform.openai.com/api-keys)
then export it as an environment variable using the following commands:

```shell
echo >> ~/.bashrc 'export OPENAI_API_KEY=YOUR_API_KEY'
exec bash
```

## Chat completions

Running just `gpt` will give you an interactive session:

```shell
$ gpt
you> You're a sorter. Reply only with sorted lists.
Understood, please provide the list you would like sorted.
you> b, c, a
a, b, c
```

You can also pipe a single prompt to stdin and get a single reply on
stdout:

```shell
$ echo >prompt.txt 'Generate TS definitions from Go structs. Just the code, no backticks:'
$ echo >api.go 'type Foo struct { Bar string `json:"bar"` }'
$ cat prompt.txt api.go | gpt | tee api.ts
interface Foo {
  bar: string;
}
```

Alternatively, you can provide the prompt as arguments. This will generate
a single reply on stdout:

```shell
$ gpt Write the ffmpeg command to trim screenrec.mp4 from 10s to 30s. Just the command, no backticks.
ffmpeg -i screenrec.mp4 -ss 00:00:10 -to 00:00:30 -c copy output.mp4
```

The default system prompt is "You are a helpful assistant." You can
customize it with `-system`:

```shell
$ gpt -system="You're a coder. No comments. No blank lines. No backticks."
you> Fisher-Yates shuffle in JS
function fisherYatesShuffle(array) {
  let m = array.length, t, i;
  while (m) {
    i = Math.floor(Math.random() * m--);
    t = array[m];
    array[m] = array[i];
    array[i] = t;
  }
  return array;
}
you>
```

## Model selection

The default model is `gpt-4o`. List available models with `-models`:

```shell
$ gpt -models
o3-mini
o1
gpt-4o
...
```

Specify a different model with `-model`. A handy way to use this is
with a shell alias:

```shell
alias o1='gpt -model=o1'
```
