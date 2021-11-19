#!/bin/bash

function check_num_locked_targets() {
  ## check number of locked targets
  locked_count=$(consul kv get -recurse gnmic/collectors/targets | wc -l) 
  expected_node_count=$1
  if [[ $locked_count -ne $expected_node_count ]]
  then
    printf "Number of locked nodes is not %s, it's %s... time to panic\n" $expected_node_count $locked_count
    exit 1
  fi
  printf "Number of locked nodes          : %s\n" $locked_count
  printf "Expected number of locked nodes : %s\n" $expected_node_count
  print_clusters
}

function print_clusters() {
  printf "Clusters:\n"
  consul kv get -recurse gnmic | awk -F: '{print $1}' | awk -F/ '{print $2}' | uniq | nl -w1 -s') '
  printf "\n"
  print_single_cluster aggregators
  print_single_cluster collectors
}

function get_instance_api_endpoint() {
  service_instance=$1"-api"
  res=$(curl -s http://127.0.0.1:8500/v1/agent/services | jq --arg si "$service_instance" '.[$si]' | jq -r '(.Address+ ":" + (.Port|tostring))')
  protocol="http://"
  for t in $(curl -s http://127.0.0.1:8500/v1/agent/services | jq --arg si "$service_instance" '.[$si]' | jq -r .Tags[] )
    do
      if [[ "$t" = protocol=* ]]
        then
          protocol=$(echo $t | awk -F= '{print $2}')
      fi
    done
  echo $protocol"://"$res
}

function print_single_cluster() {
  cluster_name=$1
  printf "Cluster name                    : %s\n" $cluster_name
  printf "Number of locked nodes          : %s\n" $(consul kv get -recurse gnmic/$cluster_name/targets | wc -l) 
  printf "gNMIc cluster leader            : %s\n" $(consul kv get -recurse gnmic/$cluster_name/leader | awk -F: '{print $2}')
  for instance in $(consul kv get -recurse gnmic/$cluster_name/targets | awk -F: '{print $2}' | sort | uniq)
    do
      api_endpoint=$(get_instance_api_endpoint $instance)
      printf "%s:\n" $instance
      printf "\t API endpoint           : %s\n" $api_endpoint
      printf "\t locked nodes           : %s\n" $(get_number_of_locked_nodes $cluster_name $instance)
      printf "\t nodes in config        : %s\n" $(get_number_of_configured_nodes $api_endpoint)
      printf "\t handled nodes          : %s\n" $(get_number_of_handled_nodes $api_endpoint)
    done
  
  printf "Instance to target mapping      :\n"
  consul kv get -recurse gnmic/$cluster_name/targets | awk -F/ '{print $4}' | awk -F: '{print "\t"$2":\t"$1}' | sort | nl -w2 -s')'
  printf "\n"
}

function get_number_of_locked_nodes() {
  cluster_name=$1
  instance=$2
  echo $(consul kv get -recurse gnmic/$cluster_name/targets | grep $instance | wc -l)
}

function get_number_of_configured_nodes() {
  api_endpoint=$1
  echo $(curl -s $api_endpoint/api/v1/config/targets | jq -r 'keys[]' | wc -l)
}

function get_number_of_handled_nodes() {
  api_endpoint=$1
  echo $(curl -s $api_endpoint/api/v1/targets | jq -r 'keys[]' | wc -l)
}
