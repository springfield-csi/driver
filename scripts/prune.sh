#!/bin/bash -x


docker image prune --all --force
docker network prune
docker container prune

