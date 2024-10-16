#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

targets=clab-test1-srl1,clab-test1-srl2,clab-test1-srl3
# create read only role
./gnmic-rc1 -u admin -p NokiaSrl1! --skip-verify --debug -a $targets -e json_ietf \
        set \
        --update-path /system/aaa/authorization \
        --update-value '{"role": {"rolename":"readonly"}}'

# craete readonly role
./gnmic-rc1 -u admin -p NokiaSrl1! --skip-verify --debug -a $targets -e json_ietf \
        set \
        --update-path /system/configuration/role[name=readonly]/rule[path-reference="/"]/action \
        --update-value "read" \
        --update-path /system/aaa/authorization/role[rolename=readonly] \
        --update-value '{"services": ["gnmi"]}'

# create a new user
./gnmic-rc1 -u admin -p NokiaSrl1! --skip-verify --debug -a $targets -e json_ietf \
        set \
        --update-path /system/aaa/authentication/user[username=user1]/password \
        --update-value "|Bo|Z%TYe*&\$P33~"

# assign readonly role to the new user
./gnmic-rc1 -u admin -p NokiaSrl1! --skip-verify --debug -a $targets -e json_ietf \
        set \
        --update-path /system/aaa/authentication/user[username=user1] \
        --update-value '{"role": ["readonly"]}'

# check user1 has access
./gnmic-rc1 -u user1 -p '|Bo|Z%TYe*&$P33~' --skip-verify --debug -a $targets -e json_ietf \
       get \
       --path /system/name

# password from ENV
GNMIC_PASSWORD="|Bo|Z%TYe*&\$P33~" ./gnmic-rc1 -u user1 --skip-verify --debug -a $targets -e json_ietf \
       get \
       --path /system/name

# Username from ENV
GNMIC_USERNAME=user1 ./gnmic-rc1 -p '|Bo|Z%TYe*&$P33~' --skip-verify --debug -a $targets -e json_ietf \
       get \
       --path /system/name

# both username and password from env
GNMIC_USERNAME=user1 GNMIC_PASSWORD='|Bo|Z%TYe*&$P33~' ./gnmic-rc1 --skip-verify --debug -a $targets -e json_ietf \
       get \
       --path /system/name

# username, password and debug from env
GNMIC_USERNAME=user1 GNMIC_PASSWORD='|Bo|Z%TYe*&$P33~' GNMIC_DEBUG=true ./gnmic-rc1 --skip-verify -a $targets -e json_ietf \
       get \
       --path /system/name

# all global flags from env
GNMIC_USERNAME=user1 GNMIC_PASSWORD='|Bo|Z%TYe*&$P33~' GNMIC_DEBUG=true GNMIC_SKIP_VERIFY=true GNMIC_ENCODING=json_ietf GNMIC_ADDRESS=$targets ./gnmic-rc1 \
       get \
       --path /system/name

## config file expansion
CUSTOM_ADDR=$targets GNMIC_USERNAME=user1 GNMIC_PASSWORD='|Bo|Z%TYe*&$P33~' GNMIC_SKIP_VERIFY=true GNMIC_ENCODING=json_ietf ./gnmic-rc1 --config configs/gnmic_env.yaml --debug \
       get \
       --path /system/name
CUSTOM_ADDR=$targets GNMIC_USERNAME=user1 GNMIC_PASSWORD='|Bo|Z%TYe*&$P33~' GNMIC_SKIP_VERIFY=true SKIPVER=false GNMIC_ENCODING=json_ietf ./gnmic-rc1 --config configs/gnmic_env.yaml --debug \
       get \
       --path /system/dns