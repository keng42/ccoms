#!/bin/bash

# go to project root
cd "$PWD/$(dirname $0)/.."

VERSION=$1
GIT_REV=$(git rev-parse --short HEAD)
BUILD_TIME=$(date +'%Y-%m-%dT%TZ%z')

pkg="github.com/keng42/ccoms/pkg/info"
vd="${VERSION}"

infoFlags="-X ${pkg}.Version=${VERSION} -X ${pkg}.GitRev=${GIT_REV} -X ${pkg}.BuildTime=${BUILD_TIME}"
alias gob='go build -ldflags "${infoFlags}" '
alias gobw='go build -ldflags "-s -w ${infoFlags}" '

if [ "$OSTYPE" = "linux-gnu" ]; then
  shopt -s expand_aliases
  source $HOME/.bashrc
fi

# disable CGO so it can run in docker's alpine image
export CGO_ENABLED=0
export GOARCH=amd64

mkdir -p ./build/ccoms-linux/app

echo "building ${vd}"

# build for linux
echo "building for linux"

export GOOS=linux
gob -o ./build/ccoms-linux/ccoms ./cmd/main

gobResult=$?
if [ "$gobResult" != "0" ]; then
  exit
fi

# package

cp README.md CHANGELOG.md RELEASELOG.md LICENSE ./build/ccoms-linux/
cp -Rf ./benchmark/ccoms-bm/* ./build/ccoms-linux/
mv ./build/ccoms-linux/ccoms ./build/ccoms-linux/app/ccoms

chmod +x ./build/ccoms-linux/ccoms

cd ./build
tar zcvf ccoms-linux.tar.gz ccoms-linux
cd ..

echo "done"
