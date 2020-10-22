Starting with `gnmic v0.4.0` release the users can enjoy the interactive prompt mode which can be enabled with the [`prompt`](../cmd/prompt.md) command.

<script id="asciicast-QaJRqrLSOGvgcAavybsMRzD7c" data-autoplay="true" data-loop="true" src="https://asciinema.org/a/QaJRqrLSOGvgcAavybsMRzD7c.js" async></script>

The prompt mode delivers two major features:

- simplifies `gnmic` commands and flags navigation, as every option is suggested and auto-completed
- provides interactive YANG path auto-suggestions for `get`, `set`, `subscribe` commands effectively making the terminal your YANG browser

## Using the prompt interface
Depending on the cursor position in the prompt line, a so-called _suggestion box_ pops up with contextual auto-completions. The user can enter the suggestion box by pressing the <kbd>TAB</kbd> key. The <kbd>↑</kbd> and <kbd>↓</kbd> keys can be used to navigate the suggestion list.

Select the suggested menu item with <kbd>SPACE</kbd> key or directly commit your command with <kbd>ENTER</kbd>, its that easy!

The following most-common key bindings will work in the prompt mode:

| Key combination                            | Description                                              |
| ------------------------------------------ | -------------------------------------------------------- |
| <kbd>Option/Control</kbd> + <kbd>→/←</kbd> | move cursor a word right/left                            |
| <kbd>Control</kbd> + <kbd>W</kbd>          | delete a word to the left                                |
| <kbd>Control</kbd> + <kbd>Z</kbd>          | delete a path element in the xpath string ([example][1]) |
| <kbd>Control</kbd> + <kbd>A</kbd>          | move cursor to the beginning of a line                   |
| <kbd>Control</kbd> + <kbd>E</kbd>          | move cursor to the end of a line                         |
| <kbd>Control</kbd> + <kbd>C</kbd>          | discard the current line                                 |
| <kbd>Control</kbd> + <kbd>D</kbd>          | exit prompt                                              |
| <kbd>Control</kbd> + <kbd>K</kbd>          | delete the line after the cursor to the clipboard        |
| <kbd>Control</kbd> + <kbd>U</kbd>          | delete the line before the cursor to the clipboard       |
| <kbd>Control</kbd> + <kbd>L</kbd>          | clear screen                                             |

## Commands and flags suggestions
To make `gnmic` configurable and flexible we introduced a considerable amount of flags and sub-commands.  
To help the users navigate the sheer selection of `gnmic` configuration options, the prompt mode will auto-suggest the global flags, sub-commands and local flags of those sub-commands.

When the prompt mode is launched, the suggestions will be shown for the top-level commands and all the global flags. Once the sub-command is typed into the terminal, the auto-suggestions will be provided for the commands nested under this command and its local flags.

In the following demo we show how the command and flag suggestions work. As the prompt starts, the suggestion box immediately hints what commands and global flags are available for input as well as their description.

The user starts with adding the global flags `--address, --insecure, --username` and then selects the `capabilities` command and commits it. This results in gNMI Capability RPC execution against a specified target.

<script id="asciicast-zsACIBIUiiyoeqgYQ82EjUCIM" src="https://asciinema.org/a/zsACIBIUiiyoeqgYQ82EjUCIM.js" async></script>

### Mixed mode
Its perfectly fine to specify some global flags outside of the prompt command and add more within the prompt mode. For example, the following is a valid invocation:

```
gnmic --insecure --username admin --password admin --address 10.1.0.11 prompt
```

Here the prompt will start with with the `insecure, username, password, address` flags set.

## YANG-completions
One of the most challenging problems in the network automation field is to process the YANG models and traverse YANG trees to construct the requests used against the network elements.  
Be it gNMI, NETCONF or RESTCONF a users still needs to have a path pointing to specific YANG-defined node which is targeted by a request.

In gNMI paths can be represented in a [human readable XPATH-like form](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-path-conventions.md#constructing-paths) - `/a/b/c[key=val]/d` - and these paths are based on the underlying YANG models.  
The problem at hand was how to get these paths interactively, or even better - walk the YANG tree from within the CLI and dynamically build the path used in a gNMI RPC?

With **YANG-completions** feature embedded in `gnmic` what used to be a dream is now a reality 🎉

<p align=center><script id="asciicast-G1O3pN7xRMLe0tqHjBvDJ7mYA" src="https://asciinema.org/a/G1O3pN7xRMLe0tqHjBvDJ7mYA.js" async></script></p>

Let us explain what just happened there.

In the demonstration above, we called the `gnmic` with the well-known flags defining the gNMI target (`address`, `username`, `password`). But this time we also added a few YANG specific flags ([`--file`](../cmd/prompt.md#file) and [`--dir`](../cmd/prompt.md#dir)) that load the full set of Nokia SR OS YANG models and the 3rd party models SR OS rely on.

```
gnmic --address 10.1.0.11 --insecure --username admin --password admin \
      --file ~/7x50_YangModels/YANG/nokia-combined \
      --dir ~/7x50_YangModels/YANG \
      prompt
```

In the background `gnmic` processed these YANG models to build the entire schema tree of the Nokia SR OS state and configuration datastores. With that in-mem stored information, `gnmic` was able to auto-suggest all the possible YANG paths when the user entered the `--path` flag which accepts gNMI paths.

By using the auto-suggestion hints, a user navigated the `/state` tree of a router and drilled down to the version-number leaf that, in the end, was retrieved with the gNMI Get RPC.

!!! success "YANG-driven path suggestions"
    `gnmic` is now capable of reading and processing YANG modules to enable live path auto-suggestions

### YANG processing
For the YANG-completion feature to work its absolutely imperative for `gnmic` to successfully parse and compile the YANG models.

The [`prompt`](../cmd/prompt.md) command leverages the [`--file`](../cmd/prompt.md#file) and [`--dir`](../cmd/prompt.md#dir) flags to select the YANG models for processing.


With the `--file` flag a user specifies a file path to a YANG file or a directory of them that `gnmic` will read and process. If it points to a directory it will be visited recursively reading in all `*.yang` files it finds.

The `--dir` flag also points to a YANG file or a directory and indicates which additional YANG files might be required. For example, if the YANG modules that a user specified with the `--file` flag import or include modules that were not part of the path specified with `--file`, they need to be added with the `--dir` flag.

The [Examples](#examples) section provide some good practical examples on how these two flags can be used together to process the YANG models from different vendors.

### Understanding path suggestions
When `gnmic` provides a user with the path suggestions it does it in a smart and intuitive way.

![path suggestions](https://gitlab.com/rdodin/pics/-/wikis/uploads/d3815b474605765989d136753c0f9c87/image.png)

First, it understands in what part of the tree a user currently is and suggests only the next possible elements.

Additionally, the suggested next path elements will be augmented with the information extracted from the YANG model, such as:

* element description, as given in the YANG `description` statement for the element
* element configuration state (`rw` / `ro`), as defined in section [4.2.3 of RFC 7950](https://tools.ietf.org/html/rfc7950#section-4.2.3).
* node type:
    * The containers and lists will be denoted with the `[+]` marker, which means that a user can type `/` char after them to receive suggestions for the nested elements.
    * the `[⋯]` character belongs to a leaf-list element.
    * an empty space will indicate the leaf element.

### Examples
The examples in this section will show how to use the `--file` and `--dir` flags of the [`prompt`](../cmd/prompt.md) command with the YANG collections from different vendors and standard bodies.

#### Nokia SR OS
YANG repo: [nokia/7x50_YangModels](https://github.com/nokia/7x50_YangModels)

Clone the repository with Nokia YANG models and checkout the release of interest:

```
git clone https://github.com/nokia/7x50_YangModels
cd 7x50_YangModels
git checkout sros_20.7.r2
```

Start `gnmic` in prompt mode and read in the nokia-combined YANG modules:

```
gnmic --file YANG/nokia-combined \
      --dir YANG \
      prompt
```

This will enable path auto-suggestions for the entire tree of the Nokia SR OS YANG models.

The full command with the gNMI target specified could look like this:

```
gnmic --address 10.1.0.11 --insecure --username admin --password admin \
      prompt \
      --file ~/7x50_YangModels/YANG/nokia-combined \
      --dir ~/7x50_YangModels/YANG
```

#### Openconfig
YANG repo: [openconfig/public](https://github.com/openconfig/public)

Clone the OpenConfig repository:

```
git clone https://github.com/openconfig/public
cd public
```

Start `gnmic` in prompt mode and read in all the modules:

```
gnmic --file release/models \
      --dir third_party \
      --exclude ietf-interfaces \
      prompt
```

<script id="asciicast-pcEq80BAs0N9RvgMLZTYJ9S8I" src="https://asciinema.org/a/pcEq80BAs0N9RvgMLZTYJ9S8I.js" async></script>

!!! note
    With OpenConfig models we have to use `--exclude` flag to exclude ietf-interfaces module from being clashed with OpenConfig interfaces module.

#### Cisco
YANG repo: [YangModels/yang](https://github.com/YangModels/yang)

Clone the `YangModels/yang` repo and change into the main directory of the repo:

```
git clone https://github.com/YangModels/yang
cd yang/vendor
```

##### IOS-XR
The IOS-XR native YANG models are disaggregated and spread all over the place. Although its technically possible to load them all in one go, this approach will produce a lot of top-level modules making the navigation quite hard.

An easier and cleaner approach would be to find the relevant module(s) and load them separately or in small batches. For example here we load BGP config and operational models together:

```
gnmic --file vendor/cisco/xr/721/Cisco-IOS-XR-um-router-bgp-cfg.yang \
      --file vendor/cisco/xr/721/Cisco-IOS-XR-ipv4-bgp-oper.yang \
      --dir standard/ietf \
      prompt
```

!!! note
    We needed to include the `ietf/` directory by means of the `--dir` flag, since the Cisco's native modules rely on the IETF modules and these modules are not in the same directory as the BGP modules.

The full command that you can against the real Cisco IOS-XR node must have a target defined, the encoding set and origin suggestions enabled. Here is what it can look like:

```
gnmic -a 10.10.30.5:57500 --insecure -e json_ietf -u admin -p Cisco123 \
      prompt \
      --file yang/vendor/cisco/xr/662/Cisco-IOS-XR-ipv4-bgp-cfg.yang \
      --file yang/vendor/cisco/xr/662/Cisco-IOS-XR-ipv4-bgp-oper.yang \
      --dir yang/standard/ietf \
      --suggest-with-origin
```

##### NX-OS
Cisco NX-OS native modules, on the other hand, are aggregated in a single file, here is how you can generate the suggestions from it:

```
gnmic --file vendor/cisco/xr/721/Cisco-IOS-XR-um-router-bgp-cfg.yang \
      --dir standard/ietf \
      prompt
```

#### Juniper
YANG repo: [Juniper/yang](https://github.com/Juniper/yang)

Clone the Juniper YANG repository and change into the release directory:

```
git clone https://github.com/Juniper/yang
cd yang/20.3/20.3R1
```

Start `gnmic` and generate path suggestions for the whole configuration tree of Juniper MX:

```
gnmic --file junos/conf --dir common prompt
```

!!! note
    1. Juniper models are constructed in a way that a top-level container appears to be `/configuration`, that will not work with your gNMI Subscribe RPC. Instead, you should omit this top level container. So, for example, the suggested path `/configuration/interfaces/interface/state` should become `/interfaces/interface/state`.
    2. Juniper vMX doesn't support gNMI Get RPC, if you plan to test it, use gNMI Subscribe RPC
    3. With gNMI Subscribe, specify `-e proto` flag to enable protobuf encoding.

#### Arista
YANG repo: [aristanetworks/yang](https://github.com/aristanetworks/yang)

Arista uses a subset of OpenConfig modules and does not provide IETF modules inside their repo. So make sure you have IETF models available so you can reference it, a `openconfig/public` is a good candidate.

Clone the Arista YANG repo:

```
git clone https://github.com/aristanetworks/yang
cd yang
```

Generate path suggestions for all Arista OpenConfig modules:

```
gnmic --file EOS-4.23.2F/openconfig/public/release/models \
      --dir ~/public/third_party/ietf \
      --exclude ietf-interfaces \
      prompt
```

## Enumeration suggestions
`gnmic` flags that can take pre-defined values (enumerations) will get suggestions as well. For example, no need to keep in mind which subscription modes are available, the prompt will hint you:

![enum suggestion](https://gitlab.com/rdodin/pics/-/wikis/uploads/a2772c709d869d5efc299db451e3d4a9/image.png)

## File-path completions
Whenever a user needs to provide a file path in a prompt mode, the filepath suggestions will make the process interactive:

<script id="asciicast-uJyTI4nnQ52lSpIw5Ec7INLe7" src="https://asciinema.org/a/uJyTI4nnQ52lSpIw5Ec7INLe7.js" async></script>

[1]: https://gitlab.com/rdodin/pics/-/wikis/uploads/cc97ef563e2b973da512951fedd1ddb8/CleanShot_2020-10-21_at_11.37.57.mp4