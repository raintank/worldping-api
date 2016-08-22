#!/bin/bash

set -x
# Find the directory we exist within
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd ${DIR}

VERSION=`git describe --always`

mkdir build
cp ../build/worldping-api build/
cp -a ../public build/
cp -a ../conf build/

docker build -t raintank/worldping-api:$VERSION .
docker tag raintank/worldping-api:$VERSION raintank/worldping-api:latest
