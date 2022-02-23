#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

DIR_NAME="$(pwd)/srl-latest-yang-models"

docker pull ghcr.io/nokia/srlinux
id=$(docker create ghcr.io/nokia/srlinux)
mkdir -p $DIR_NAME
docker cp $id:/opt/srlinux/models/. $DIR_NAME
docker rm $id
ls -l srl-latest-yang-models

./gnmic-rc1 generate --path /interface/subinterface --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools."
./gnmic-rc1 generate --path /interface/subinterface --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools." --camel-case
./gnmic-rc1 generate --path /interface/subinterface --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools." --snake-case

./gnmic-rc1 generate --path /network-instance/protocols/bgp --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools."
#./gnmic-rc1 generate --path /network-instance/protocols/bgp --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools." --camel-case
./gnmic-rc1 generate --path /network-instance/protocols/bgp --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools." --snake-case

./gnmic-rc1 generate --path /interface/subinterface --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools." --config-only
./gnmic-rc1 generate --path /interface/subinterface --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools." --config-only --camel-case
./gnmic-rc1 generate --path /interface/subinterface --file  srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models/ietf --exclude ".tools." --config-only --snake-case
