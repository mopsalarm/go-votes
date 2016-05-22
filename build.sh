#!/bin/sh
set -e

# install go dependencies
go get github.com/Masterminds/glide
glide install

# build binary
go build
