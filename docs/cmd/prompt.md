## Description
The `[prompt]` command starts an interactive gnmic prompt in which the same gNMI commands (`capabilities`, `get`, `set`, `subscribe`) can be executed with some additional auto completion features:

* Auto-completion for all commands names and their flags.
* Auto-completion for flags with xpath values ( `--path`, `--prefix`, `--model`,...). The xpaths are generated from models suplied using `prompt` command `--file` and `--dir` (optional) flags.
* Auto-completion for flags with enum values `--format`, `--encoding`, ...
* Auto-completion for flags taking file or directory values.


### Usage

`gnmic [global-flags] prompt [local-flags]`


### Flags 

#### file

File or directories pointing to yang modules from which to generate gnmi paths.

Multiple `--file` flags can be supplied.

#### dir

Direcotries where to search for yang modules included and imported in modules under `--file`

Multiple `--dir` flags can be supplied.

#### exclude

The `--exclude` flag is used to specify the yang modules to be excluded from path generation

Multiple `--exclude` flags can be supplied.

#### max-suggestions

The `--max-suggestions` flag is used to limit the size the the terminal box showing command completion suggestions. (uint16)

#### description-bg-color

The `--description-bg-color` flag is used to change the background color of the description part of the suggestions descriptions

#### prefix-color

The `--prefix-color` flag is used to change the gnmic prompt prefix color `gnmic> `.

Defaults to ??

#### suggestions-bg-color

The `--suggestions-bg-color` flag is used to change the suggestions background color.

Defaults to ??

### Examples


