#!/bin/sh

go build && ./gg-deployer --debug  -c ./gg-deployer.config.json
