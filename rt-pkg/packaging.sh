#!/bin/bash
set -x 
# Find the directory we exist within
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd ${DIR}

NAME=worldping-api
VERSION=$(git describe --long)
BUILD="${DIR}/${NAME}-${VERSION}"
ARCH="$(uname -m)"
PACKAGE_NAME="${DIR}/artifacts/${NAME}-${VERSION}_${ARCH}.deb"
GOBIN="${DIR}/.."


mkdir -p ${BUILD}/usr/share/${NAME}
mkdir -p ${BUILD}/etc/init
mkdir -p ${BUILD}/etc/raintank
mkdir -p ${BUILD}/usr/sbin
cp -a ${DIR}/artifacts/conf ${BUILD}/usr/share/${NAME}/
cp -a ${DIR}/artifacts/public ${BUILD}/usr/share/${NAME}/
cp ${DIR}/artifacts/bin/worldping-api ${BUILD}/usr/sbin/

cp ${DIR}/artifacts/conf/sample.ini ${BUILD}/etc/raintank/worldping-api.ini

fpm -s dir -t deb \
  -v ${VERSION} -n ${NAME} -a ${ARCH} --description "Worldping Backend service" \
  --deb-upstart ${DIR}/conf/worldping-api \
  -C ${BUILD} -p ${PACKAGE_NAME} .
