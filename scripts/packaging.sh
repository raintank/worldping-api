#!/bin/bash
set -x
BASE=$(dirname $0)
CODE_DIR=$(readlink -e "$BASE/../")

BUILD_ROOT=$CODE_DIR/build

ARCH="$(uname -m)"
VERSION=$(git describe --long --always)
NAME=worldping-api


## ubuntu 14.04
BUILD=${BUILD_ROOT}/upstart

mkdir -p ${BUILD}/usr/share/${NAME}
mkdir -p ${BUILD}/usr/sbin
mkdir -p ${BUILD}/etc/init
mkdir -p ${BUILD}/etc/raintank

cp ${CODE_DIR}/conf/sample.ini ${BUILD}/etc/raintank/worldping-api.ini
cp ${BUILD_ROOT}/worldping-api ${BUILD}/usr/sbin/
cp -a ${CODE_DIR}/public ${BUILD}/usr/share/${NAME}
cp -a ${CODE_DIR}/conf ${BUILD}/usr/share/${NAME}

PACKAGE_NAME="${BUILD}/${NAME}-${VERSION}_${ARCH}.deb"
fpm -s dir -t deb \
  -v ${VERSION} -n ${NAME} -a ${ARCH} --description "Worldping Backend service" \
  --config-files /etc/raintank/ \
  --deb-upstart ${BASE}/etc/upstart/worldping-api \
  -m "Raintank Inc. <hello@raintank.io>" --vendor "raintank.io" \
  --license "Apache2.0" -C ${BUILD} -p ${PACKAGE_NAME} .

## ubuntu 16.04
BUILD=${BUILD_ROOT}/systemd
mkdir -p ${BUILD}/usr/share/${NAME}
mkdir -p ${BUILD}/usr/sbin
mkdir -p ${BUILD}/lib/systemd/system/
mkdir -p ${BUILD}/etc/raintank

cp ${CODE_DIR}/conf/sample.ini ${BUILD}/etc/raintank/worldping-api.ini
cp ${BUILD_ROOT}/worldping-api ${BUILD}/usr/sbin/
cp -a ${CODE_DIR}/public ${BUILD}/usr/share/${NAME}
cp -a ${CODE_DIR}/conf ${BUILD}/usr/share/${NAME}
cp ${BASE}/etc/systemd/worldping-api.service ${BUILD}/lib/systemd/system/

PACKAGE_NAME="${BUILD}/${NAME}-${VERSION}_${ARCH}.deb"
fpm -s dir -t deb \
  -v ${VERSION} -n ${NAME} -a ${ARCH} --description "Worldping Backend service" \
  --config-files /etc/raintank/ \
  -m "Raintank Inc. <hello@raintank.io>" --vendor "raintank.io" \
  --license "Apache2.0" -C ${BUILD} -p ${PACKAGE_NAME} .
