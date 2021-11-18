#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

# single host, single path
./gnmic-rc1 -a clab-test1-srl1 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name
./gnmic-rc1 -a clab-test1-srl2 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name
./gnmic-rc1 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name

# multiple hosts, single path
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name
#
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name

# multiple hosts, multiple paths
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name \
                              --path /interface[name=*]
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name \
                              --path /interface[name=*]
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name \
                              --path /interface[name=*]
#
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name \
                              --path /interface[name=*]
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name \
                              --path /interface[name=*]
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -u admin -p admin --skip-verify -d -e ascii subscribe \
                              --mode once \
                              --path /system/name \
                              --path /interface[name=*]

# subscription config from file
./gnmic-rc1 -a clab-test1-srl1 --config configs/gnmic1.yaml subscribe
./gnmic-rc1 -a clab-test1-srl2 --config configs/gnmic1.yaml subscribe
./gnmic-rc1 -a clab-test1-srl3 --config configs/gnmic1.yaml subscribe
./gnmic-rc1 -a clab-test1-srl1 --config configs/gnmic1.yaml subscribe --format prototext
./gnmic-rc1 -a clab-test1-srl1 --config configs/gnmic1.yaml subscribe --format protojson
./gnmic-rc1 -a clab-test1-srl1 --config configs/gnmic1.yaml subscribe --format event

# hosts and config from file
./gnmic-rc1 --config configs/gnmic2.yaml subscribe
./gnmic-rc1 --config configs/gnmic2.yaml subscribe --format prototext
./gnmic-rc1 --config configs/gnmic2.yaml subscribe --format protojson
./gnmic-rc1 --config configs/gnmic2.yaml subscribe --format event

# nodes from targets field
./gnmic-rc1 --config configs/gnmic3.yaml subscribe 
./gnmic-rc1 --config configs/gnmic3.yaml subscribe --format prototext 
./gnmic-rc1 --config configs/gnmic3.yaml subscribe --format protojson 
./gnmic-rc1 --config configs/gnmic3.yaml subscribe --format event

# multiple once subscriptions
./gnmic-rc1 --config configs/gnmic4.yaml subscribe
./gnmic-rc1 --config configs/gnmic4.yaml subscribe --format prototext
./gnmic-rc1 --config configs/gnmic4.yaml subscribe --format protojson
./gnmic-rc1 --config configs/gnmic4.yaml subscribe --format event