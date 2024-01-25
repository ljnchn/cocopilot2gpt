#!/bin/bash

# Run make release with the specified version
make release VERSION=v0.5

# Build the docker image
sudo docker build -t copilot2gpt .

# Tag the docker image
sudo docker tag copilot2gpt ersichub/copilot2gpt:latest

# Push the docker image to Docker Hub
sudo docker push ersichub/copilot2gpt:latest