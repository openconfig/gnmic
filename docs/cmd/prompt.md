## Description
The `[prompt]` command starts an interactive gnmic prompt in which the same gNMI commands (`capabilities`, `get`, `set`, `subscribe`) can be executed with some additional auto completion features:

* Auto-completion for all commands names and their flags.
* Auto-completion for flags with xpath values ( `--path`, `--prefix`, `--model`,...). The xpaths are generated from models suplied using `prompt` command `--file` and `--dir` (optional) flags.
* Auto-completion for flags with enum values `--format`, `--encoding`, ...
* Auto-completion for flags taking file or directory values.


### Usage

`gnmic [global-flags] prompt [local-flags]`

### Examples
