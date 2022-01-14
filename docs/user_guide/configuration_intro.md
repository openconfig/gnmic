`gnmic` reads configuration from three different sources,
[Global and local flags](configuration_flags.md), [environment variables](configuration_env.md) and [local system file](configuration_file.md).

The different sources follow a precedence order where a configuration variable from a source take precedence over the next one in the below list:

- global and local flags
- Environment variables
- configuration file

## Flags

See [here](configuration_flags.md) for a complete list of the supported global and local flags.

## Environment variables

`gnmic` can also be configured using environment variables, it will read the environment variables starting with `GNMIC_`.

The Env variable names are inline with the flag names as well as the configuration hierarchy.

See [here](configuration_env.md) for more details on environment variables.
## File configuration
Configuration file that `gnmic` reads must be in one of the following formats: JSON, YAML, TOML, HCL or Properties.  

By default, `gnmic` will search for a file named `.gnmic.[yml/yaml, toml, json]` in the following locations and will use the first file that exists:

* `$PWD`
* `$HOME`
* `$XDG_CONFIG_HOME`
* `$XDG_CONFIG_HOME/gnmic`

The default path can be overridden with [`--config`](../global_flags.md#config) flag.

```bash
# config file default path is :
# $PWD/.gnmic.[yml, toml, json], or
# $HOME/.gnmic.[yml, toml, json], or
# $XDG_CONFIG_HOME/.gnmic.[yml, toml, json], or
# $XDG_CONFIG_HOME/gnmic/.gnmic.[yml, toml, json]
gnmic capabilities

# read `cfg.yml` file located in the current directory
gnmic --config ./cfg.yml capabilities
```

If the file referenced by `--config` flag is not present, the default path won't be tried.

Example of the `gnmic` config files are provided in the following formats: [YAML](https://github.com/karimra/gnmic/blob/main/config.yaml), [JSON](https://github.com/karimra/gnmic/blob/main/config.json), [TOML](https://github.com/karimra/gnmic/blob/main/config.toml).