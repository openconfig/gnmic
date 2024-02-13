#!/bin/bash

set -e

function testmodule
{
    cd $1
    go test -cover ./...
    cd $SCRIPTPATH/..
}

declare -a modules=("." "pkg/api" "pkg/cache")

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

cd $SCRIPTPATH/..

for i in "${modules[@]}"
do
    echo "Running tests for module $i"
    testmodule "$i"
done
