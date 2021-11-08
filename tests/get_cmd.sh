#!/bin/bash 

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

# get
$gnmic_base_cmd -a clab-test1-srl1 -e json_ietf get \
                              --path /system/name/host-name
$gnmic_base_cmd -a clab-test1-srl2 -e json_ietf get \
                              --path /system/name/host-name
$gnmic_base_cmd -a clab-test1-srl3 -e json_ietf get \
                              --path /system/name/host-name

# get multi paths
$gnmic_base_cmd -a clab-test1-srl1 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
$gnmic_base_cmd -a clab-test1-srl2 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
$gnmic_base_cmd -a clab-test1-srl3 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server

# comma separated paths
$gnmic_base_cmd -a clab-test1-srl1 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
$gnmic_base_cmd -a clab-test1-srl2 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
$gnmic_base_cmd -a clab-test1-srl3 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server

# get multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -e json_ietf get \
                              --path /system/name/host-name
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -e json_ietf get \
                               --path /system/name/host-name
# get multi hosts and paths
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -e json_ietf get \
                              --path /system/name/host-name \
                              --path /system/gnmi-server
