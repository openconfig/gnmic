`gnmic` supports various Inputs to consume gnmi data, transform it and ultimately export it to one or multiple Outputs.

The purpose of `gnmic`'s Inputs is to build a gnmi data pipeline by enabling the ingestion and export of gnmi data that was exported by `gnmic`'s outputs upstream.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/gnmic_inputs_intro&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fgnmic_inputs_intro" async></script>


Currently supported input types:

* [NATS messaging system](nats_input.md)
* [NATS Streaming messaging bus (STAN)](stan_input.md)
* [Kafka messaging bus](kafka_input.md)

### Defining Inputs and matching Outputs

To define an Input a user needs to fill in the `inputs` section in the configuration file.

Each Input is defined by its name (`input1` in the example below), a `type` field which determines the type of input to be created (`nats`, `stan`, `kafka`) and various other configuration fields which depend on the Input type.

!!! note
    Inputs names are case insensitive

All Input types have an `outputs` field, under which the user can defined the downstream destination(s) of the consumed data.
This way, data consumed once, can be exported multiple times.

!!!info
    The same `gnmic` instance can act as gNMI collector, input and output simultaneously.

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

#### Clustering
Using `gnmic` Inputs, the user can aggregate all the collected data into one instance of `gnmic` that can make it available to a downstream off the shelf tool,typically Prometheus.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams//gnmic_inputs_clustering&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2F/gnmic_inputs_clustering" async></script>


#### Data reuse
Collect data once and use it multiple times. By chaining multiple instances of `gnmic` the user can process the same stream of data in different ways.

A different set of event processors can be applied on the data stream before being exported to its intended outputs.


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/gnmic_input_data_reuse&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fgnmic_input_data_reuse" async></script>


