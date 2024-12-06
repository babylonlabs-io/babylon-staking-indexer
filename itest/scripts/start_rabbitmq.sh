#!/bin/bash

# Check if the RabbitMQ container is already running
RABBITMQ_CONTAINER_NAME="rabbitmq"
if [ $(docker ps -q -f name=^/${RABBITMQ_CONTAINER_NAME}$) ]; then
    echo "RabbitMQ container already running. Skipping RabbitMQ startup."
else
    echo "Starting RabbitMQ"
    # Start RabbitMQ
    docker compose up -d rabbitmq
fi
