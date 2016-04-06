#!/bin/bash
set -x 
# Find the directory we exist within
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd ${DIR}

NAME=worldping-api
VERSION="0.0.2" # need an automatic way to do this again :-/
BUILD="${DIR}/${NAME}-${VERSION}"
ARCH="$(uname -m)"
PACKAGE_NAME="${DIR}/artifacts/${NAME}-VERSION-ITERATION_ARCH.deb"
GOBIN="${DIR}/.."
ITERATION=`date +%s`ubuntu1
TAG="pkg-${VERSION}-${ITERATION}"

git tag $TAG

mkdir -p ${BUILD}/usr/share/${NAME}
mkdir -p ${BUILD}/etc/init
mkdir -p ${BUILD}/etc/raintank
mkdir -p ${BUILD}/usr/sbin
cp -a ${DIR}/artifacts/conf ${BUILD}/usr/share/${NAME}/
cp -a ${DIR}/artifacts/public ${BUILD}/usr/share/${NAME}/
cp ${DIR}/artifacts/bin/worldping-api ${BUILD}/usr/sbin/

cp ${DIR}/artifacts/conf/sample.ini ${BUILD}/etc/raintank/worldping-api.ini

fpm -s dir -t deb \
  -v ${VERSION} -n ${NAME} -a ${ARCH} --iteration $ITERATION --description "Worldping Backend service" \
  --deb-upstart ${DIR}/conf/worldping-api \
  -C ${BUILD} -p ${PACKAGE_NAME} .
