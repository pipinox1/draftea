package infrastructure

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/draftea/payment-system/shared/events"
	"github.com/pkg/errors"
)

// SNSPublisherAdapter adapts SNSEventPublisher to work with events.Publisher interface
type SNSPublisherAdapter struct {
	snsPublisher *SNSEventPublisher
}

// NewSNSPublisherAdapter creates a new SNS publisher adapter
func NewSNSPublisherAdapter(topicArn string) (*SNSPublisherAdapter, error) {
	// Load AWS config (works with LocalStack when AWS_ENDPOINT_URL is set)
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AWS config")
	}

	// Create SNS client
	snsClient := sns.NewFromConfig(cfg)

	// Create SNS publisher
	snsPublisher := NewSNSEventPublisher(snsClient, topicArn)

	return &SNSPublisherAdapter{
		snsPublisher: snsPublisher,
	}, nil
}

// Publish implements events.Publisher interface
func (p *SNSPublisherAdapter) Publish(ctx context.Context, events ...*events.Event) error {
	return p.snsPublisher.Publish(ctx, events...)
}

// Close closes the publisher
func (p *SNSPublisherAdapter) Close() error {
	// SNS client doesn't need explicit closing
	return nil
}