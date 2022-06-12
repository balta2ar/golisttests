# golisttests

A helper that lists test names starting from the current directory. Used to provide completion candidates when integrating with fzf.

## How it works

There are two approaches:
1. At first, I used to traverse AST directly using stdlib APIs. That worked for some of the cases and was fine for a while.
2. Then I discovered tree-sitter and used their API to discover test names in the AST.

Both approaches are currently used in the codebase.