#!/bin/sh
set -e

# install go dependencies
go get github.com/Masterminds/glide
glide install

# build binary
CGO_ENABLED=0 go build -a

# build docker image
docker build -t pr0gramm/go-votes .
