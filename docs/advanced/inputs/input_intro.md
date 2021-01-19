`gnmic` supports various Inputs to consume gnmi data, transform it and ultimately export it to one or multiple Outputs.

The purpose of `gnmic`'s Inputs is to build a gnmi data pipeline by enabling the ingestion and export of gnmi data that was exported by `gnmic`'s outputs upstream.

<!-- DIAGRAM_PLACEHOLDER -->

Currently supported input types:

* [NATS messaging system](nats_input.md)
* [NATS Streaming messaging bus (STAN)](stan_input.md)
* [Kafka messaging bus](kafka_input.md)

### Defining Inputs and matching Outputs

To define an Input a user needs to fill in the `inputs` section in the configuration file.

Each Input is defined by its name (`input1` in the example below), a `type` field which determines the type of input to be created (`nats`, `stan`, `kafka`) and various other configuration fields which depend on the Input type.

All Input types have an `outputs` field, under which the user can defined the downstream destination(s) of the consumed data.
This way, data consumed once, can be exported multiple times.

Example:

```yaml
# part of gnmic config file
inputs:
  input1:
    type: nats # input type
    #
    # other config fields depending on the input type
    #
    outputs:
      - output1
      - output2
```

### Inputs use cases


