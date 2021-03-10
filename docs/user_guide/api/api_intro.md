A limited set of REST endpoints are supported, these are mainly used to allow for a clustered deployment for multiple `gnmic` instances.

The API can be used to automate (to a certain extent) the targets configuration loading and starting/stopping subscriptions.

Enabling the API server can be done via a command line flag:
```bash
gnmic --config gnmic.yaml subscribe --api ":7890"
```
via ENV variable: `GNMIC_API=':7890'`

Or via file configuration, by adding the below line to the config file:

```yaml
api: ":7890"
```

## API Endpoints

* [Configuration](./configuration.md)

* [Targets](./targets.md)

