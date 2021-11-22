#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

curl -sS http://$1/api/v1/config | yq eval -P
curl -sS http://$1/api/v1/config/targets | yq eval -P
for i in $(curl -sS http://$1/api/v1/config/targets | jq -r 'keys[]');
    do
        curl -sS http://$1/api/v1/config/targets/$i |yq eval -P
    done

curl -sS http://$1/api/v1/config/subscriptions | yq eval -P
curl -sS http://$1/api/v1/config/outputs | yq eval -P
curl -sS http://$1/api/v1/config/inputs | yq eval -P
curl -sS http://$1/api/v1/config/processors | yq eval -P
curl -sS http://$1/api/v1/config/clustering | yq eval -P
curl -sS http://$1/api/v1/config/api-server | yq eval -P
curl -sS http://$1/api/v1/config/gnmi-server | yq eval -P
curl -sS http://$1/api/v1/targets | yq eval -P

for i in $(curl -sS http://$1/api/v1/targets | jq -r 'keys[]');
    do
        curl -sS http://$1/api/v1/targets/$i | yq eval -P
    done

curl -sS http://$1/api/v1/cluster | yq eval -P
curl -sS http://$1/api/v1/cluster/members | yq eval -P
curl -sS http://$1/api/v1/cluster/leader | yq eval -P
