# golisttests

A helper that lists test names starting from the current directory. Used to provide completion candidates when integrating with fzf.

## How it works

There are two approaches:
1. At first, I used to traverse AST directly using stdlib APIs. That worked for some of the cases and was fine for a while.
2. Then I discovered tree-sitter and used their API to discover test names in the AST.

Both approaches are currently used in the codebase.

## fzf integration

```bash
_fzf_complete_go() {
  ARGS="$@"
  if [[ $ARGS == 'go test'* ]]; then
    _fzf_complete "--no-sort --info=inline" "$@" < <(
      { golisttests -limit -maxFiles 10000 -maxExecution 1s }
    )
  else
    eval "zle ${fzf_default_completion:-expand-or-complete}"
  fi
}

_fzf_complete_go_post() {
  cut -f1 -d' '
}
```