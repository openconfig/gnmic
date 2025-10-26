#!/bin/bash

set -e

function testmodule
{
    cd $1
    go test -cover ./... -v -count=1
    cd $SCRIPTPATH/..
}

declare -a modules=("." "pkg/api" "pkg/cache", "pkg/inputs/jetstream_input", "pkg/outputs/nats_outputs/jetstream")

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

cd $SCRIPTPATH/..

for i in "${modules[@]}"
do
    echo "Running tests for module $i"
    testmodule "$i"
done
