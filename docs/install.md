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

To upgrade the tool run the installation script once again, it will perform the upgrade if a newer version is available.

### Windows
It is highly recommended to use [WSL](https://en.wikipedia.org/wiki/Windows_Subsystem_for_Linux) on Windows, but if its not possible, use [releases page](https://github.com/karimra/gnmic/releases) to download the windows executable file.

### Docker
The `gnmic` Dockerfile is [provided](https://github.com/karimra/gnmic/blob/master/Dockerfile) within the repository for a manual build process. Later the ready-made docker image will be part of the release pipeline and you will be able to pull the image from the docker hub.

Alternatively you can install `gnmic` in docker container in an ad-hoc fashion:

```bash
# launch a docker container based with a libc (debian/rhel)
docker run -it centos:7
# install the gnmic
sudo curl -sL https://github.com/karimra/gnmic/raw/master/install.sh | sudo bash
```