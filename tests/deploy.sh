#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

# printf "Installing containerlab...\n"
# bash -c "$(curl -sL https://get-clab.srlinux.dev)"
sudo clab version
printf "\n"
printf "Deploying lab $1\n"
sudo clab deploy -t clab/$1.clab.yaml --reconfigure
