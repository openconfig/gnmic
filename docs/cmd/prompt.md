## Description
The `prompt` command starts `gnmic` in an interactive prompt mode with the following auto-completion features:

* All `gnmic` [commands names and their flags are suggested](../advanced/prompt_suggestions.md#commands-and-flags-suggestions).
* Values for the flags that rely on YANG-defined data (like `--path`, `--prefix`, `--model`,...) will be dynamically suggested, we call this feature [YANG-completions](../advanced/prompt_suggestions.md#yang-completions).  
The auto-completions are generated from the YANG modules d with the `--file` and `--dir` flags.
* Flags with the fixed set of values (`--format`, `--encoding`, ...) will get their [values suggested](../advanced/prompt_suggestions.md#enumeration-suggestions).
* Flags that require a [file path value will auto-suggest](../advanced/prompt_suggestions.md#file-path-completions) the available files as the user types.


### Usage

`gnmic [global-flags] prompt [local-flags]`

### Flags

#### file
A path to a YANG file or a directory with YANG files which `gnmic` will use to generate auto-completion for YANG-defined data (paths, models).

Multiple `--file` flags can be supplied.

#### dir
A path to a directory which `gnmic` would recursively traverse in search for the additional YANG files which may be required by YANG files specified with `--file` to build the YANG tree.

Can also point to a single YANG file instead of a directory.

Multiple `--dir` flags can be supplied.

#### description-with-prefix
When set, the description field of the suggestion box will have a module prefix name before the element description.

#### description-with-types
When set, the description field of the suggestion box will have a YANG type information provided for the elements.

#### exclude
The `--exclude` flag specifies the YANG module __names__ to be excluded from the path generation when YANG modules names are clashed.

Multiple `--exclude` flags can be supplied.

#### max-suggestions
The `--max-suggestions` flag sets the number of lines that the suggestion box will display without scrolling.

Defaults to 10.

#### suggest-all-flags
The `--suggest-all-flags` makes `gnmic` prompt to suggest both global and local flags for a sub-command.

The default behavior (when this flag is not set) is to suggest global flags only on prompt start, and suggest only local flags for any typed sub-command.

#### suggestions-bg-color
The `--suggestions-bg-color` flag sets the background color of the left part of the suggestion box.

Defaults to dark blue.

#### description-bg-color
The `--description-bg-color` flag sets the background color of the right part of the suggestion box.

Defaults to dark gray.

#### prefix-color
The `--prefix-color` flag sets the gnmic prompt prefix color `gnmic> `.

Defaults to dark blue.

### Examples
The detailed explanation of the prompt command the the YANG-completions is provided on the [Prompt mode and auto-suggestions](../advanced/prompt_suggestions.md) page.
