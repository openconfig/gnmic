#!/bin/bash 

# get
./gnmic-rc1 -a clab-test1-srl1 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name
./gnmic-rc1 -a clab-test1-srl2 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name
./gnmic-rc1 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name

# get multi paths
./gnmic-rc1 -a clab-test1-srl1 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
./gnmic-rc1 -a clab-test1-srl2 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
./gnmic-rc1 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server

# comma separated paths
./gnmic-rc1 -a clab-test1-srl1 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
./gnmic-rc1 -a clab-test1-srl2 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
./gnmic-rc1 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server

# get multi hosts
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                               --path /system/name/host-name
# get multi hosts and paths
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server