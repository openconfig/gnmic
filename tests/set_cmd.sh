#!/bin/bash

gnmic_base_cmd="./gnmic-rc1 -u admin -p admin --skip-verify -d"

#########
## SET ##
#########
#################
## SET REPLACE ##
#################

### set replace with value
#### single host

$gnmic_base_cmd -a clab-test1-srl1 set \
                              --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1 set \
                              --delimiter "CUSTOM_DELIMITER" \
                              --replace /interface[name=ethernet-1/1]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-1_dummy_desc1
                              
$gnmic_base_cmd -a clab-test1-srl1 set -e json_ietf \
                              --replace-path /interface[name=ethernet-1/1]/description \
                              --replace-value e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl2 set \
                              --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl2 set \
                              --delimiter "CUSTOM_DELIMITER" \
                              --replace /interface[name=ethernet-1/1]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl2 set -e json_ietf \
                              --replace-path /interface[name=ethernet-1/1]/description \
                              --replace-value e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl3 set \
                              --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl3 set \
                              --delimiter "CUSTOM_DELIMITER" \
                              --replace /interface[name=ethernet-1/1]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl3 set -e json_ietf \
                              --replace-path /interface[name=ethernet-1/1]/description \
                              --replace-value e1-1_dummy_desc1

#### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set \
                              --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -e json_ietf set \
                              --replace-path /interface[name=ethernet-1/1]/description \
                              --replace-value e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set \
                              --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set \
                              --delimiter "CUSTOM_DELIMITER" \
                              --replace /interface[name=ethernet-1/1]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -e json_ietf set \
                              --replace-path /interface[name=ethernet-1/1]/description \
                              --replace-value e1-1_dummy_desc1

### set replace with multiple values
#### single host
$gnmic_base_cmd -a clab-test1-srl1 set \
                          --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                          --replace /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1 set \
                          --delimiter "CUSTOM_DELIMITER" \
                          --replace /interface[name=ethernet-1/1]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-1_dummy_desc1 \
                          --replace /interface[name=ethernet-1/2]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-2_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1 -e json_ietf set \
                          --replace-path /interface[name=ethernet-1/1]/description \
                          --replace-value e1-1_dummy_desc1 \
                          --replace-path /interface[name=ethernet-1/2]/description \
                          --replace-value e1-2_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl2 set \
                          --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                          --replace /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl2 set \
                          --delimiter "CUSTOM_DELIMITER" \
                          --replace /interface[name=ethernet-1/1]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-1_dummy_desc1 \
                          --replace /interface[name=ethernet-1/2]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-2_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl2 -e json_ietf set \
                          --replace-path /interface[name=ethernet-1/1]/description \
                          --replace-value e1-1_dummy_desc1 \
                          --replace-path /interface[name=ethernet-1/2]/description \
                          --replace-value e1-2_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl3 set \
                          --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                          --replace /interface[name=ethernet-1/2]/description:::json_ietf:::e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl3 set \
                          --delimiter "CUSTOM_DELIMITER" \
                          --replace /interface[name=ethernet-1/1]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-1_dummy_desc1 \
                          --replace /interface[name=ethernet-1/2]/descriptionCUSTOM_DELIMITERjson_ietfCUSTOM_DELIMITERe1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl3 -e json_ietf set \
                          --replace-path /interface[name=ethernet-1/1]/description \
                          --replace-value e1-1_dummy_desc1 \
                          --replace-path /interface[name=ethernet-1/2]/description \
                          --replace-value e1-2_dummy_desc1


#### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set \
                          --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -e json_ietf set \
                          --replace-path /interface[name=ethernet-1/1]/description \
                          --replace-value e1-1_dummy_desc1

$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set \
                          --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1
                          
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -e json_ietf set \
                          --replace-path /interface[name=ethernet-1/1]/description \
                          --replace-value e1-1_dummy_desc1

### set replace with file
#### JSON file
##### single host
cat configs/node/interface.json
$gnmic_base_cmd -a clab-test1-srl1 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.json

$gnmic_base_cmd -a clab-test1-srl2 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.json 

$gnmic_base_cmd -a clab-test1-srl3 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.json

##### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.json
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.json

#### YAML file
##### single host
cat configs/node/interface.yaml
$gnmic_base_cmd -a clab-test1-srl1 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.yaml
$gnmic_base_cmd -a clab-test1-srl2 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.yaml
$gnmic_base_cmd -a clab-test1-srl3 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.yaml
##### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.yaml
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 -e json_ietf -d set \
                          --replace-path /interface[name=ethernet-1/1] \
                          --replace-file configs/node/interface.yaml 

### set replace with request file
# TODO

################
## SET UPDATE ##
################

### set update with value
#### single host
$gnmic_base_cmd -a clab-test1-srl1 set --update /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1
$gnmic_base_cmd -a clab-test1-srl1 set --update-path /interface[name=ethernet-1/1]/description --update-value e1-1_dummy_desc2 -e json_ietf
$gnmic_base_cmd -a clab-test1-srl2 set --update /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1
$gnmic_base_cmd -a clab-test1-srl2 set --update-path /interface[name=ethernet-1/1]/description --update-value e1-1_dummy_desc2 -e json_ietf
$gnmic_base_cmd -a clab-test1-srl3 set --update /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1
$gnmic_base_cmd -a clab-test1-srl3 set --update-path /interface[name=ethernet-1/1]/description --update-value e1-1_dummy_desc2 -e json_ietf
#### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set --update /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set --update-path /interface[name=ethernet-1/1]/description --update-value e1-1_dummy_desc1 -e json_ietf
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set --update /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set --update-path /interface[name=ethernet-1/1]/description --update-value e1-1_dummy_desc1 -e json_ietf

### set update with file
#### JSON file
##### single host
cat configs/node/interface.json
$gnmic_base_cmd -a clab-test1-srl1 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.json -e json_ietf
$gnmic_base_cmd -a clab-test1-srl2 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.json -e json_ietf
$gnmic_base_cmd -a clab-test1-srl3 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.json -e json_ietf
##### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.json -e json_ietf
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.json -e json_ietf
#### YAML file
##### single host
cat configs/node/interface.yaml
$gnmic_base_cmd -a clab-test1-srl1 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.yaml -e json_ietf
$gnmic_base_cmd -a clab-test1-srl2 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.yaml -e json_ietf
$gnmic_base_cmd -a clab-test1-srl3 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.yaml -e json_ietf
##### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.yaml -e json_ietf
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set --update-path /interface[name=ethernet-1/1] --update-file configs/node/interface.yaml -e json_ietf
### set update with request file
# TODO

## delete
### single host
$gnmic_base_cmd -a clab-test1-srl1 set --delete /interface[name=ethernet-1/1]/description
$gnmic_base_cmd -a clab-test1-srl2 set --delete /interface[name=ethernet-1/1]/description
$gnmic_base_cmd -a clab-test1-srl3 set --delete /interface[name=ethernet-1/1]/description
### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set --delete /interface[name=ethernet-1/1]/description
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set --delete /interface[name=ethernet-1/1]/description

## combined update, replace and delete
# TODO