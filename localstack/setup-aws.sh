#!/bin/bash

set -e

echo "Setting up AWS resources in LocalStack..."

# Set AWS CLI configuration for LocalStack
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

# LocalStack endpoint
LOCALSTACK_ENDPOINT="http://localstack:4566"

echo "Creating SNS topic: payment-events"
aws --endpoint-url=$LOCALSTACK_ENDPOINT sns create-topic --name payment-events

echo "Creating SQS queue: payment-events"
aws --endpoint-url=$LOCALSTACK_ENDPOINT sqs create-queue --queue-name payment-events

echo "Creating SQS DLQ: payment-events-dlq"
aws --endpoint-url=$LOCALSTACK_ENDPOINT sqs create-queue --queue-name payment-events-dlq

echo "Subscribing SQS queue to SNS topic"
aws --endpoint-url=$LOCALSTACK_ENDPOINT sns subscribe \
    --topic-arn arn:aws:sns:us-east-1:000000000000:payment-events \
    --protocol sqs \
    --notification-endpoint arn:aws:sqs:us-east-1:000000000000:payment-events

# Set up DLQ policy for the main queue
echo "Configuring DLQ policy"
aws --endpoint-url=$LOCALSTACK_ENDPOINT sqs set-queue-attributes \
    --queue-url http://localstack:4566/000000000000/payment-events \
    --attributes '{
        "RedrivePolicy": "{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:payment-events-dlq\",\"maxReceiveCount\":3}"
    }'

echo "AWS resources setup completed successfully!"

# List created resources
echo "=== Created Resources ==="
echo "SNS Topics:"
aws --endpoint-url=$LOCALSTACK_ENDPOINT sns list-topics

echo "SQS Queues:"
aws --endpoint-url=$LOCALSTACK_ENDPOINT sqs list-queues

echo "=== Setup Complete ==="