# Change Log

## 2020-10-05 Added prompt mode

- Added prompt-mode subcommand to root command.
- Separated flags initialization func to reset the used subcommand flags in the prompt mode.
- Updated path command to generate a schema fil ($HOME/.gnmic.schema) for tab completion of the prompt mode.
- Added --dir, --file and --exclude flags to the prompt mode for single step schema loading.
- Add command history to the prompt mode

## Jobs to do for gnmic prompt mode

This prompt mode is still being developed. The following work items will be implemented.

- subcommand execution must return success or failure information to the terminal. (e.g. `get` doesn't any info upon `get` operation failure.)
- `Subscribe` subcommand must be run in background on the prompt mode.
- boolean or enum values must be presented on the tab completion list.