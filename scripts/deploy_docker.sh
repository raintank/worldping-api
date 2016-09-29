#!/bin/bash

set -x
# Find the directory we exist within
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd ${DIR}

VERSION=`git describe --always --long`

docker push raintank/worldping-api:$VERSION
docker push raintank/worldping-api:latest
