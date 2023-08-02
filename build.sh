#!/bin/sh

APP=gnmic
go build -v 

[ ! -x ${APP} ] && exit 1

tar -cSpf ${APP}.tar ${APP} ${APP}.spec

if [ ! -d ${HOME}/rpm ]; then echo "Run mkrpmenv first, your build environment is lacking."; exit 1; fi
rpmbuild -tb ${APP}.tar && rm -rf ${APP}.tar
echo "build done"

