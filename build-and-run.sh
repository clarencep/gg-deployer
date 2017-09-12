#!/bin/sh

go build && exec ./gg-deployer --debug  -c ./gg-deployer.config.json
