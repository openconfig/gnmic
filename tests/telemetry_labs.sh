#!/bin/bash

export SHELLOPTS
set -eET

failure() {
  local lineno=$1
  local msg=$2
  echo "Failed at line $lineno: $msg"
}

NUM_LABS=5
NUM_NODES_PER_LAB=14

export -f failure

function cleanup() {
    printf "cleaning up...\n"
    cd clab/telemetry
    sudo clab destroy -t telemetry.clab.yaml --cleanup
    cd ../..
    #
    for i in `seq 1 $NUM_LABS`
    do
      printf "destroying lab clab/lab%s.clab.yaml\n" $i
      sudo clab destroy -t clab/lab$i.clab.yaml --cleanup
      rm clab/lab$i.clab.yaml
      rm -rf .lab$i.clab.yaml
    done
}

source ./cluster_funcs.sh

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR
trap cleanup EXIT
trap cleanup SIGINT

start=`date +%s`
# generate lab files
for i in `seq 1 $NUM_LABS`
  do 
    echo 'ID: ' $i | gomplate -f clab/labN.clab.yaml -d data=stdin:///id.yaml -o clab/lab${i}.clab.yaml
  done

# destroy labs if they are still up.
for i in `seq 1 $NUM_LABS`
  do
    sudo clab destroy -t clab/lab${i}.clab.yaml --cleanup 
  done


# build docker image
docker build -t gnmic:0.0.0-rc1 ../

# deploy telemetry lab
echo ""
cd clab/telemetry
sudo clab deploy -t telemetry.clab.yaml --reconfigure
cd ../..

echo ""
# check all containers are running
container_count=$(docker ps -f label=containerlab=telemetry -q | wc -l)
if [ $container_count -ne 13 ]
  then
    printf "Number of telemetry containers is not 13, it's %s... time to panic\n" $container_count
    exit 1
fi

printf "Found %s running containers\n" $container_count

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

##################################
#  Deploying labs with SRL nodes #
##################################
while true
do
printf "Waiting a bit before deploying the nodes\n"
echo ""
sleep 10

for i in `seq 1 $NUM_LABS`
  do
    echo "Deploying lab" $i
    sudo clab deploy -t clab/lab${i}.clab.yaml --reconfigure
  done
echo ""

sleep 30
nodes_count=$(docker ps -f label=clab-node-kind=srl -f label=test=telemetry -q | wc -l)
printf "Found %s running SRL nodes\n" $nodes_count

printf "Waiting for the next docker loader run before checking the number of locked targets...\n"
sleep 30

check_num_locked_targets $(($NUM_NODES_PER_LAB * $NUM_LABS))
sleep 60

echo "Running API calls..."
./api.sh clab-telemetry-gnmic1:7890
./api.sh clab-telemetry-gnmic2:7891
./api.sh clab-telemetry-gnmic3:7892
./api.sh clab-telemetry-agg-gnmic1:7893
./api.sh clab-telemetry-agg-gnmic2:7894
./api.sh clab-telemetry-agg-gnmic3:7895

echo ""
#start adding and removing labs
echo "Waiting a bit before starting to add and remove labs..."
sleep 10
## remove 2 labs
sudo clab destroy -t clab/lab1.clab.yaml --cleanup
sudo clab destroy -t clab/lab5.clab.yaml --cleanup
sleep 60

check_num_locked_targets $(($NUM_NODES_PER_LAB * ((${NUM_LABS} - 2))))

sleep 60
## add 1 lab
echo "Re Deploying lab1"
sudo clab deploy -t clab/lab1.clab.yaml --reconfigure
sleep 60

check_num_locked_targets $(($NUM_NODES_PER_LAB * ((${NUM_LABS} - 1))))
sleep 60

## add 1 lab and remove 1
echo "Destroying lab2, Adding back lab5"
sudo clab deploy -t clab/lab5.clab.yaml --reconfigure
sudo clab destroy -t clab/lab2.clab.yaml --cleanup
sleep 60

check_num_locked_targets $(($NUM_NODES_PER_LAB * ((${NUM_LABS} - 1))))
sleep 60

echo "Running API calls..."
./api.sh clab-telemetry-gnmic1:7890
./api.sh clab-telemetry-gnmic2:7891
./api.sh clab-telemetry-gnmic3:7892
./api.sh clab-telemetry-agg-gnmic1:7893
./api.sh clab-telemetry-agg-gnmic2:7894
./api.sh clab-telemetry-agg-gnmic3:7895

echo "Re Deploying lab2"
sudo clab deploy -t clab/lab2.clab.yaml --reconfigure
sleep 60
check_num_locked_targets $(($NUM_NODES_PER_LAB * $NUM_LABS))

for i in `seq 1 $NUM_LABS`
    do
      printf "destroying lab clab/lab%s.clab.yaml\n" $i
      sudo clab destroy -t clab/lab$i.clab.yaml --cleanup
      # rm clab/lab$i.clab.yaml
      # rm -rf .lab$i.clab.yaml
    done
sleep 60
done
#######
# END #
#######

# calculate runtime
end=`date +%s`
runtime=$((end-start))
printf "runtime=%ss\n" $runtime
