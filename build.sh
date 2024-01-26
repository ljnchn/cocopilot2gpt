#!/bin/bash

export VERSION=$1

if [ -z "$VERSION" ]; then
    echo "No version"
    exit 1
fi


# Run make release with the specified version
make release VERSION="$VERSION"

# Build the docker image
sudo docker build -t copilot2gpt .

# Tag the docker image
sudo docker tag copilot2gpt ersichub/copilot2gpt:latest

# Push the docker image to Docker Hub
sudo docker push ersichub/copilot2gpt:latest