#!/bin/bash

# Check if the MongoDB container is already running
MONGO_CONTAINER_NAME="indexer-mongodb"
if [ $(docker ps -q -f name=^/${MONGO_CONTAINER_NAME}$) ]; then
    echo "Indexer mongoDB container already running. Skipping MongoDB startup."
else
    echo "Starting indexer mongoDB"
    # Start indexer mongoDB
    docker compose up -d indexer-mongodb
fi

# Check if the RabbitMQ container is already running
RABBITMQ_CONTAINER_NAME="rabbitmq"
if [ $(docker ps -q -f name=^/${RABBITMQ_CONTAINER_NAME}$) ]; then
    echo "RabbitMQ container already running. Skipping RabbitMQ startup."
else
    echo "Starting RabbitMQ"
    # Start RabbitMQ
    docker compose up -d rabbitmq
    # Wait for RabbitMQ to start
    sleep 10
fi