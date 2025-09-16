package infrastructure

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/draftea/payment-system/shared/events"
	"github.com/pkg/errors"
)

// SQSSubscriberAdapter adapts SQSEventSubscriber to work with events.Subscriber interface
type SQSSubscriberAdapter struct {
	sqsSubscriber *SQSEventSubscriber
	isRunning     bool
	queueURL      string
}

// NewSQSSubscriberAdapter creates a new SQS subscriber adapter
func NewSQSSubscriberAdapter(queueURL string) (*SQSSubscriberAdapter, error) {
	return &SQSSubscriberAdapter{
		sqsSubscriber: nil, // Will be created when Subscribe is called
		isRunning:     false,
		queueURL:      queueURL,
	}, nil
}

// eventHandlerAdapter adapts events.EventHandler to work with SQS EventHandler
type eventHandlerAdapter struct {
	handler events.EventHandler
}

func (a *eventHandlerAdapter) HandlerID() string {
	// Use a default handler ID since the original interface doesn't provide one
	return "event-handler-adapter"
}

func (a *eventHandlerAdapter) Handle(ctx context.Context, event *events.Event) error {
	// No conversion needed anymore since we're using the unified Event type
	return a.handler.Handle(ctx, event)
}

// Subscribe implements events.Subscriber interface
func (s *SQSSubscriberAdapter) Subscribe(ctx context.Context, eventType string, handler events.EventHandler) error {
	if s.isRunning {
		return errors.New("subscriber is already running")
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to load AWS config")
	}

	// Create SQS client
	sqsClient := sqs.NewFromConfig(cfg)

	// Create adapted handler
	adaptedHandler := &eventHandlerAdapter{handler: handler}

	// Create SQS subscriber using the configured queue URL
	s.sqsSubscriber = NewSQSEventSubscriber(sqsClient, s.queueURL, adaptedHandler)

	// Start the subscriber
	if err := s.sqsSubscriber.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start SQS subscriber")
	}

	s.isRunning = true
	return nil
}

// Close stops the subscriber
func (s *SQSSubscriberAdapter) Close() error {
	if !s.isRunning || s.sqsSubscriber == nil {
		return nil
	}

	ctx := context.Background()
	if err := s.sqsSubscriber.Stop(ctx); err != nil {
		return errors.Wrap(err, "failed to stop SQS subscriber")
	}

	s.isRunning = false
	return nil
}