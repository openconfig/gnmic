#!/bin/bash

gnmic_base_cmd="./gnmic-rc1 -u admin -p admin --skip-verify -d"

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

# capabilities
$gnmic_base_cmd -a clab-test1-srl1 capabilities
$gnmic_base_cmd -a clab-test1-srl2 capabilities
$gnmic_base_cmd -a clab-test1-srl3 capabilities
# capabilities multi host
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 capabilities
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 capabilities --format json
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 capabilities
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 capabilities --format json

printf "capabilities with config file\n"
# capabilities using config file
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl1 capabilities
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl2 capabilities
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl3 capabilities
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl1 capabilities --no-prefix
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl2 capabilities --no-prefix
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl3 capabilities --no-prefix
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl1 capabilities --format json
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl2 capabilities --format json
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl3 capabilities --format json
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl1 capabilities --format json --no-prefix
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl2 capabilities --format json --no-prefix
./gnmic-rc1 --config configs/gnmic1.yaml -a clab-test1-srl3 capabilities --format json --no-prefix
# multi host
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml capabilities
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml capabilities --no-prefix
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml capabilities --format json
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml capabilities --format json --no-prefix
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 --config configs/gnmic1.yaml capabilities
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 --config configs/gnmic1.yaml capabilities --no-prefix
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 --config configs/gnmic1.yaml capabilities --format json
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 --config configs/gnmic1.yaml capabilities --format json --no-prefix

## hosts from file
### target nodes in address field
./gnmic-rc1 --config configs/gnmic2.yaml capabilities
./gnmic-rc1 --config configs/gnmic2.yaml capabilities --format json
./gnmic-rc1 --config configs/gnmic2.yaml capabilities --format json --no-prefix
### target nodes in targets field
./gnmic-rc1 --config configs/gnmic3.yaml capabilities
./gnmic-rc1 --config configs/gnmic2.yaml capabilities --format json
./gnmic-rc1 --config configs/gnmic3.yaml capabilities --format json --no-prefix


# set skip-verify value to false in the config file
sed -i 's/^skip-verify: true/skip-verify: false/g' configs/gnmic1.yaml
./gnmic-rc1 -a clab-test1-srl1 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl2 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities --format json
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities --format json

# comment out skip-verify value in the config file and change it to true
sed -i 's/^skip-verify: false/#skip-verify: true/g' configs/gnmic1.yaml
./gnmic-rc1 -a clab-test1-srl1 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl2 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities

./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities --format json

./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities
./gnmic-rc1 -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 --config configs/gnmic1.yaml --skip-verify capabilities --format json

# use --tls-ca
./gnmic-rc1 -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 --config configs/gnmic1.yaml \
                                                               --tls-ca clab-test1/ca/root/root-ca.pem \
                                                               capabilities
# revert back skip-verify value to true
sed -i 's/^#skip-verify: true/skip-verify: true/g' configs/gnmic1.yaml



