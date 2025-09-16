#!/bin/bash

echo "LocalStack initialization script started..."

# Wait for LocalStack to be ready
echo "Waiting for LocalStack to be ready..."
while ! curl -f http://localhost:4566/health > /dev/null 2>&1; do
    echo "Waiting for LocalStack..."
    sleep 2
done

echo "LocalStack is ready!"

# Set AWS CLI configuration for LocalStack
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
export AWS_ENDPOINT_URL=http://localhost:4566

echo "LocalStack initialization completed!"