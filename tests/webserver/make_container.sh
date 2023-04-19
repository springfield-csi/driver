#!/bin/bash -x


docker logout ghcr.io

export VERSION="0.1.0"

# TODO: Add logic to only build the container when necessary.
export TZ="EST"
docker build --no-cache -f Dockerfile --build-arg  BUILD_DATE="$(date)" -t springfield-test-webserver:devel .

echo $GITHUB_PAT | docker login ghcr.io -u trgill --password-stdin

docker tag springfield-test-webserver:devel ghcr.io/trgill/springfield-test-webserver:devel
docker push ghcr.io/trgill/springfield-test-webserver:devel

