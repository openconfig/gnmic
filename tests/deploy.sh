#!/bin/bash

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

sh "$project_dir/scripts/containerlab.sh" version
printf "\n"
printf "Deploying lab $1\n"
sh "$project_dir/scripts/containerlab.sh" deploy -t clab/$1.clab.yaml --reconfigure
