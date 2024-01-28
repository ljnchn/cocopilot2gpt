#!/bin/bash
source .env
if [ -z "$VERSION" ]
then
    echo "VERSION is empty"
    exit 1
else
    echo "VERSION is $VERSION"
fi


# Run make release with the specified version
make release VERSION="$VERSION"

# Build the docker image
sudo docker build -t copilot2gpt .

# Tag the docker image
sudo docker tag copilot2gpt ersichub/copilot2gpt:"$VERSION"

# Push the docker image to Docker Hub
sudo docker push ersichub/copilot2gpt:"$VERSION"

# Tag the docker image
sudo docker tag copilot2gpt ersichub/copilot2gpt:latest

# Push the docker image to Docker Hub
sudo docker push ersichub/copilot2gpt:latest