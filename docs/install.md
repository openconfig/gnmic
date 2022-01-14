`gnmic` is a single binary built for the Linux, Mac OS and Windows operating systems distributed via [Github releases](https://github.com/karimra/gnmic/releases).

### Linux/Mac OS

To download & install the latest release the following automated [installation script](https://github.com/karimra/gnmic/blob/main/install.sh) can be used:

```bash
bash -c "$(curl -sL https://get-gnmic.kmrd.dev)"
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
bash -c "$(curl -sL https://get-gnmic.kmrd.dev)" -- -v 0.5.0
```

#### Packages

Linux users running distributions with support for `deb`/`rpm` packages can install `gnmic` using pre-built packages:

```bash
bash -c "$(curl -sL https://get-gnmic.kmrd.dev)" -- --use-pkg
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
# pull a specific release from dockerhub
docker pull gnmic/gnmic:0.7.0

# pull latest release from github registry
docker pull ghcr.io/karimra/gnmic:latest
# pull a specific release from github registry
docker pull ghcr.io/karimra/gnmic:0.5.2
```

Example running `gnmic get` command using the docker image:
```bash
docker run \
       --network host \
       --rm ghcr.io/karimra/gnmic get --log --username admin --password admin --insecure --address router1.local --path /interfaces
```

### Docker Compose

`gnmic` docker-compose file example:

```yaml
version: '2'

networks:
  gnmic-net:
    driver: bridge

services:
  gnmic-1:
    image: ghcr.io/karimra/gnmic:latest
    container_name: gnmic-1
    networks:
      - gnmic-net
    volumes:
      - ./gnmic.yaml:/app/gnmic.yaml
    command: "subscribe --config /app/gnmic.yaml"
```

See [here](deployments/deployments_intro.md) for more deployment options
