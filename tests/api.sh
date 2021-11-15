#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

curl -s http://$1/config | jq
curl -s http://$1/config/targets | jq
for i in $(curl -s http://$1/config/targets | jq -r 'keys[]');
    do
        curl -s http://$1/config/targets/$i | jq
    done
curl -s http://$1/config/subscriptions | jq
curl -s http://$1/config/outputs | jq
curl -s http://$1/config/inputs | jq
curl -s http://$1/config/processors | jq
curl -s http://$1/config/clustering | jq
curl -s http://$1/config/api-server | jq
curl -s http://$1/config/gnmi-server | jq
curl -s http://$1/targets | jq
for i in $(curl -s http://$1/targets | jq -r 'keys[]');
    do
        curl -s http://$1/targets/$i | jq
    done