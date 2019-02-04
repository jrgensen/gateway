#!/bin/sh
cd src
go get -v -d
go get github.com/canthefason/go-watcher
go install github.com/canthefason/go-watcher/cmd/watcher
watcher
