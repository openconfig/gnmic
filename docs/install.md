`gnmic` is a single binary built for the Linux, Mac OS and Windows platforms distributed via [Github releases](https://github.com/karimra/gnmic/releases).

### Linux/Mac OS
To download & install the latest release the following automated [installation script](https://github.com/karimra/gnmic/blob/master/install.sh) can be used:

```bash
sudo curl -sL https://github.com/karimra/gnmic/raw/master/install.sh | sudo bash
```

As a result, the latest `gnmic` version will be installed in the `/usr/local/bin` directory and the version information will be printed out.
```text
Downloading gnmic_0.0.3_Darwin_x86_64.tar.gz...
Moving gnmic to /usr/local/bin

version : 0.0.3
 commit : f541948
   date : 2020-04-23T12:06:07Z
 gitURL : https://github.com/karimra/gnmic.git
   docs : https://gnmic.kmrd.dev

Installation complete!
```

To install a specific version of `gnmic`, provide the version with `-v` flag to the installation script:
```bash
sudo curl -sL https://github.com/karimra/gnmic/raw/master/install.sh | sudo bash -s -- -v 0.5.0
```

#### Packages
Linux users running distributives with support for `deb`/`rpm` packages can install `gnmic` using pre-built packages:

```
sudo curl -sL https://github.com/karimra/gnmic/raw/master/install.sh | sudo bash -s -- --use-pkg
```

#### Upgrade

To upgrade `gnmic` to the latest version use the `upgrade` command:
```bash
# upgrade using binary file
gnmic version upgrade

# upgrade using package
gnmic version upgrade --use-pkg
```

### Windows
Windows users should use [WSL](https://en.wikipedia.org/wiki/Windows_Subsystem_for_Linux) on Windows and install the linux version of the tool.

### Docker
The `gnmic` container image can be pulled from Dockerhub or GitHub container registries. The tag of the image corresponds to the release version and `latest` tag points to the latest available release:

```bash
# pull latest release from dockerhub
docker pull gnmic/gnmic:latest

# pull the specific release from github registry
docker pull ghcr.io/karimra/gnmic:0.5.2
```
