#!/usr/bin/env bash

if ! hash go; then
    echo "you need Go"
    exit
fi

if ! hash rerun; then
    go get -u -f -v github.com/skelterjohn/rerun
fi

if ! hash humanlog; then
    go get -u -f -v github.com/aybabtme/humanlog/...
fi

rerun github.com/aybabtme/fail.run/svc/cmd/failrund -port 3000 2>&1 | humanlog
