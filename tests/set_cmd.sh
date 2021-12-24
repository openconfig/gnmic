#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

#########
## SET ##
#########
#################
## SET REPLACE ##
#################

### set replace with value
#### single host
########################
$gnmic_base_cmd -a clab-test1-srl1 set \
                              -e json_ietf \
                              --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1
out=$($gnmic_base_cmd -a clab-test1-srl1 get -e json_ietf \
                        --path /interface[name=ethernet-1/1]/description | 
                        jq '.[].updates[].values."srl_nokia-interfaces:interface/description"')
contains $out "e1-1_dummy_desc1"
########################
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
                          --replace /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1

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
$gnmic_base_cmd -a clab-test1-srl1 set --request-file configs/node/replace_request_file.yaml
$gnmic_base_cmd -a clab-test1-srl2 set --request-file configs/node/replace_request_file.yaml
$gnmic_base_cmd -a clab-test1-srl3 set --request-file configs/node/replace_request_file.yaml

$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set --request-file configs/node/replace_request_file.yaml
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set --request-file configs/node/replace_request_file.yaml

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
$gnmic_base_cmd -a clab-test1-srl1 set --request-file configs/node/update_request_file.yaml
$gnmic_base_cmd -a clab-test1-srl2 set --request-file configs/node/update_request_file.yaml
$gnmic_base_cmd -a clab-test1-srl3 set --request-file configs/node/update_request_file.yaml

$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set --request-file configs/node/replace_request_file.yaml
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set --request-file configs/node/replace_request_file.yaml

## delete
### single host
$gnmic_base_cmd -a clab-test1-srl1 set --delete /interface[name=ethernet-1/1]/description
$gnmic_base_cmd -a clab-test1-srl2 set --delete /interface[name=ethernet-1/1]/description
$gnmic_base_cmd -a clab-test1-srl3 set --delete /interface[name=ethernet-1/1]/description
### multi hosts
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set --delete /interface[name=ethernet-1/1]/description
$gnmic_base_cmd -a clab-test1-srl1 -a clab-test1-srl2 -a clab-test1-srl3 set --delete /interface[name=ethernet-1/1]/description

## combined update, replace and delete
### combined set with value

$gnmic_base_cmd -a clab-test1-srl1 set \
                --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state

$gnmic_base_cmd -a clab-test1-srl2 set \
                --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state

$gnmic_base_cmd -a clab-test1-srl3 set \
                --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state

# reset
$gnmic_base_cmd -a clab-test1-srl1,clab-test1-srl2,clab-test1-srl3 set \
                --delete /interface[name=ethernet-1/1]/description \
                --delete /interface[name=ethernet-1/2]/description

$gnmic_base_cmd -a clab-test1-srl1 set \
                --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                --replace /interface[name=ethernet-1/1]/subinterface[index=0]/description:::json_ietf:::e1-1.0_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/subinterface[index=0]/description:::json_ietf:::e1-2._dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state \
                --delete /interface[name=ethernet-1/2]/admin-state

$gnmic_base_cmd -a clab-test1-srl2 set \
                --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                --replace /interface[name=ethernet-1/1]/subinterface[index=0]/description:::json_ietf:::e1-1.0_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/subinterface[index=0]/description:::json_ietf:::e1-2._dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state \
                --delete /interface[name=ethernet-1/2]/admin-state

$gnmic_base_cmd -a clab-test1-srl3 set \
                --replace /interface[name=ethernet-1/1]/description:::json_ietf:::e1-1_dummy_desc1 \
                --replace /interface[name=ethernet-1/1]/subinterface[index=0]/description:::json_ietf:::e1-1.0_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/description:::json_ietf:::e1-2_dummy_desc1 \
                --update /interface[name=ethernet-1/2]/subinterface[index=0]/description:::json_ietf:::e1-2._dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state \
                --delete /interface[name=ethernet-1/2]/admin-state

$gnmic_base_cmd -a clab-test1-srl1 set -e json_ietf \
                --replace-path /interface[name=ethernet-1/1]/description \
                --replace-value e1-1_dummy_desc1 \
                --replace-path /interface[name=ethernet-1/1]/subinterface[index=0]/description \
                --replace-value e1-1.0_dummy_desc1 \
                --update-path /interface[name=ethernet-1/2]/description \
                --update-value e1-2_dummy_desc1 \
                --update-path /interface[name=ethernet-1/2]/subinterface[index=0]/description \
                --update-value e1-2.0_dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state \
                --delete /interface[name=ethernet-1/2]/admin-state

$gnmic_base_cmd -a clab-test1-srl2 set -e json_ietf \
                --replace-path /interface[name=ethernet-1/1]/description \
                --replace-value e1-1_dummy_desc1 \
                --replace-path /interface[name=ethernet-1/1]/subinterface[index=0]/description \
                --replace-value e1-1.0_dummy_desc1 \
                --update-path /interface[name=ethernet-1/2]/description \
                --update-value e1-2_dummy_desc1 \
                --update-path /interface[name=ethernet-1/2]/subinterface[index=0]/description \
                --update-value e1-2.0_dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state \
                --delete /interface[name=ethernet-1/2]/admin-state

$gnmic_base_cmd -a clab-test1-srl3 set -e json_ietf \
                --replace-path /interface[name=ethernet-1/1]/description \
                --replace-value e1-1_dummy_desc1 \
                --replace-path /interface[name=ethernet-1/1]/subinterface[index=0]/description \
                --replace-value e1-1.0_dummy_desc1 \
                --update-path /interface[name=ethernet-1/2]/description \
                --update-value e1-2_dummy_desc1 \
                --update-path /interface[name=ethernet-1/2]/subinterface[index=0]/description \
                --update-value e1-2.0_dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state \
                --delete /interface[name=ethernet-1/2]/admin-state \
                --dry-run

$gnmic_base_cmd -a clab-test1-srl3 set -e json_ietf \
                --replace-path /interface[name=ethernet-1/1]/description \
                --replace-value e1-1_dummy_desc1 \
                --replace-path /interface[name=ethernet-1/1]/subinterface[index=0]/description \
                --replace-value e1-1.0_dummy_desc1 \
                --update-path /interface[name=ethernet-1/2]/description \
                --update-value e1-2_dummy_desc1 \
                --update-path /interface[name=ethernet-1/2]/subinterface[index=0]/description \
                --update-value e1-2.0_dummy_desc1 \
                --delete /interface[name=ethernet-1/1]/admin-state \
                --delete /interface[name=ethernet-1/2]/admin-state
