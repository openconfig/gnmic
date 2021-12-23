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
    ./cleanup.sh test_lab1
}
export -f cleanup

function contains() {
  if [[ "$1" != *"$2"* ]]; then
    exit 1
  fi
}
export -f contains

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR
trap cleanup EXIT
trap cleanup SIGINT

export gnmic_base_cmd="./gnmic-rc1 -u admin -p admin --skip-verify --debug"

function buildgNMIc() {
  printf "Building gnmic...\n"
  CGO_ENABLED=0 go build -o gnmic-rc1 -ldflags="-s -w -X 'github.com/karimra/gnmic/app.commit=$(git rev-parse --short HEAD)' -X 'github.com/karimra/gnmic/app.date=$(date)'" ../
}

start=`date +%s`

case "$1" in
  "all")
    # build gnmic
    buildgNMIc
    # run version cmd
    ./version_cmd.sh
    # run generate cmd
    ./generate_cmd.sh
    # run generate path cmd
    ./generate_path_cmd.sh
    
    # deploy basic 3 nodes lab
    ./deploy.sh test_lab1
    # run capabilities cmd tests
    ./capabilities_cmd.sh
    # run get cmd tests
    ./get_cmd.sh

    # redeploy to avoid getting error: rpc error: code = ResourceExhausted desc = Max number of operations per minute (rate-limit) reached (max: 60)
    ./deploy.sh test_lab1
    # run set md tests
    ./set_cmd.sh

    # redeploy to avoid getting error: rpc error: code = ResourceExhausted desc = Max number of operations per minute (rate-limit) reached (max: 60)
    ./deploy.sh test_lab1
    # run subscribe once cmd tests 
    ./subscribe_once_cmd.sh
    # cleanup test_lab1
    cleanup test_lab1
    # run loaders tests
    ./loaders.sh
    ;;
  "version")
    # build gnmic
    buildgNMIc

    # run version cmd
    ./version_cmd.sh
    ;;
  "generate")
    # build gnmic
    buildgNMIc
    # run generate cmd
    ./generate_cmd.sh
    ./generate_path_cmd.sh
    ;;
  "cap")
    # build gnmic
    buildgNMIc

    # deploy basic 3 nodes lab
    ./deploy.sh test_lab1

    # run capabilities cmd tests
    ./capabilities_cmd.sh
    ;;
  "get")
    # build gnmic
    buildgNMIc
    # deploy basic 3 nodes lab
    ./deploy.sh test_lab1
    # run get cmd tests
    ./get_cmd.sh
    ;;
  "set")
    # build gnmic
    buildgNMIc
    # deploy basic 3 nodes lab
    ./deploy.sh test_lab1
    # run set md tests
    ./set_cmd.sh
    ;;
  "sub")
    # build gnmic
    buildgNMIc
    # deploy basic 3 nodes lab
    ./deploy.sh test_lab1
    # run sub tests
    ./subscribe_once_cmd.sh
    ;;
  "env")
    # build gnmic
    buildgNMIc
    # deploy basic 3 nodes lab
    ./deploy.sh test_lab1
    # run sub tests
    ./env_vars.sh
    ;;
  "loaders")
    ./loaders.sh
    ;;
  *)
    echo "./run.sh [ all | version | generate | cap | get | set | sub | loaders ]"
    exit 1
    ;;
esac

# calculate runtime
end=`date +%s`
runtime=$((end-start))
printf "runtime=%ss\n" $runtime
