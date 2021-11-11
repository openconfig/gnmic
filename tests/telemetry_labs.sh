#!/bin/bash

export SHELLOPTS
set -eET

failure() {
  local lineno=$1
  local msg=$2
  echo "Failed at line $lineno: $msg"
}

export -f failure

function cleanup() {
    printf "cleaning up...\n"
    cd clab/telemetry
    sudo clab destroy -t telemetry.clab.yaml --cleanup
    cd ../..
    #
    sudo clab destroy -t clab/lab1.clab.yaml --cleanup &
    sudo clab destroy -t clab/lab2.clab.yaml --cleanup &
    sudo clab destroy -t clab/lab3.clab.yaml --cleanup &
    sudo clab destroy -t clab/lab4.clab.yaml --cleanup &
    sudo clab destroy -t clab/lab5.clab.yaml --cleanup
}

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR
#trap cleanup EXIT
trap cleanup SIGINT

start=`date +%s`

# destroy labs if they are still up.
sudo clab destroy -t clab/lab1.clab.yaml --cleanup &
sudo clab destroy -t clab/lab2.clab.yaml --cleanup & 
sudo clab destroy -t clab/lab3.clab.yaml --cleanup &
sudo clab destroy -t clab/lab4.clab.yaml --cleanup &
sudo clab destroy -t clab/lab5.clab.yaml --cleanup

# build docker image
docker build -t gnmic:0.0.0-rc1 ../

# deploy telemetry lab
echo ""
cd clab/telemetry
sudo clab deploy -t telemetry.clab.yaml --reconfigure
cd ../..

echo ""
# check all containers are running
container_count=$(docker ps -f label=containerlab=telem -q | wc -l)
if [ $container_count -ne 11 ]
  then
    printf "Number of telemetry containers is not 11, it's %s... time to panic\n" $container_count
    exit 1
fi

printf "Found 11 running containers\n"

echo ""
consul catalog services

echo ""
echo "Waiting for services to register..."
sleep 30
printf "Consul services:\n"
consul catalog services -tags

echo ""
printf "Consul services to instances:\n"
consul-template -template="consul_templates/all_services.tpl:all_services.txt" -once
cat all_services.txt
rm all_services.txt

printf "Waiting a bit before deploying the nodes\n"
echo ""
sleep 10

echo "Deploying lab1"
sudo clab deploy -t clab/lab1.clab.yaml --reconfigure &
echo "Deploying lab2"
sudo clab deploy -t clab/lab2.clab.yaml --reconfigure &
echo "Deploying lab3"
sudo clab deploy -t clab/lab3.clab.yaml --reconfigure &
echo "Deploying lab4"
sudo clab deploy -t clab/lab4.clab.yaml --reconfigure &
echo "Deploying lab5"
sudo clab deploy -t clab/lab5.clab.yaml --reconfigure 
echo ""

sleep 20
nodes_count=$(docker ps -f label=clab-node-kind=srl -q | wc -l)
printf "Found %s running SRL nodes\n" $nodes_count

printf "Waiting for the next docker loader run before checking the number of locked targets...\n"
sleep 30

printf "There are %s locked targets\n" $(consul kv get -recurse gnmic/cluster2/targets | wc -l)
locked_count=$(consul kv get -recurse gnmic/cluster2/targets | wc -l)

if [ $locked_count -ne 70 ]
  then
    printf "Number of locked nodes is not 70, it's %s... time to panic\n" $locked_count
    exit 1
fi

consul kv get -recurse gnmic/cluster2/targets | awk -F: '{print $2"\t"$1}' | sort
echo ""
printf "The cluster leader is %s\n" $(consul kv get -recurse gnmic/cluster2/leader | awk -F: '{print $2}')
printf "Instance clab-telem-gnmic1 subscribed to %s nodes\n" $(consul kv get -recurse gnmic/cluster2/targets | grep gnmic1 | wc -l)
printf "Instance clab-telem-gnmic2 subscribed to %s nodes\n" $(consul kv get -recurse gnmic/cluster2/targets | grep gnmic2 | wc -l)
printf "Instance clab-telem-gnmic3 subscribed to %s nodes\n" $(consul kv get -recurse gnmic/cluster2/targets | grep gnmic3 | wc -l)

echo "Running API calls..."
./api.sh clab-telem-gnmic1:7890
./api.sh clab-telem-gnmic2:7891
./api.sh clab-telem-gnmic3:7892
#./api.sh clab-telem-gnmic-agg:7893

#### END ####
# calculate runtime
end=`date +%s`
runtime=$((end-start))
printf "runtime=%ss\n" $runtime
