package infrastructure

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/draftea/payment-system/shared/events"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var _ events.Publisher = (*SNSEventPublisher)(nil)

const maxBatchSize = 10

type snsMessage struct {
	ID        string          `json:"id"`
	Metadata  events.Metadata `json:"metadata"`
	Topic     string          `json:"topic"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// SNSEventPublisher implements EventPublisher using AWS SNS
type SNSEventPublisher struct {
	client   *sns.Client
	topicArn string
}

// NewSNSEventPublisher creates a new SNSEventPublisher
func NewSNSEventPublisher(client *sns.Client, topicArn string) *SNSEventPublisher {
	return &SNSEventPublisher{
		client:   client,
		topicArn: topicArn,
	}
}

// Publish publishes events to SNS
func (p *SNSEventPublisher) Publish(ctx context.Context, evts ...*events.Event) error {
	if len(evts) == 0 {
		return nil
	}

	// Split into batches
	batchEvents := splitToChunks(evts, maxBatchSize)

	gr, ctx := errgroup.WithContext(ctx)

	for _, eventBatch := range batchEvents {
		eventBatch := eventBatch
		gr.Go(func() error {
			return p.batchPublish(ctx, eventBatch)
		})
	}

	return gr.Wait()
}

func (p *SNSEventPublisher) batchPublish(ctx context.Context, events []*events.Event) error {
	requests := make([]types.PublishBatchRequestEntry, len(events))

	for i, event := range events {
		payload, err := event.MarshalPayload()
		if err != nil {
			return errors.Wrap(err, "failed to marshal payload")
		}

		message := &snsMessage{
			ID:        event.ID.String(),
			Metadata:  event.Metadata,
			Topic:     string(event.Topic),
			Payload:   payload,
			Timestamp: event.Timestamp,
		}

		msgJson, err := json.Marshal(message)
		if err != nil {
			return errors.Wrap(err, "failed to marshal message")
		}

		attrs := map[string]types.MessageAttributeValue{
			"topic": {
				DataType:    aws.String("String"),
				StringValue: aws.String(string(event.Topic)),
			},
		}

		for k, v := range event.Metadata {
			if k == "sqs_message_id" || k == "sqs_receipt_handle" {
				continue
			}

			attrs[k] = types.MessageAttributeValue{
				DataType:    aws.String("String"),
				StringValue: aws.String(v),
			}
		}

		requests[i] = types.PublishBatchRequestEntry{
			Id:                aws.String(event.ID.String()),
			Message:           aws.String(string(msgJson)),
			MessageAttributes: attrs,
		}
	}

	res, err := p.client.PublishBatch(
		ctx,
		&sns.PublishBatchInput{
			TopicArn:                   &p.topicArn,
			PublishBatchRequestEntries: requests,
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to publish batch to SNS")
	}

	// Log metrics (simplified version without DataDog)
	for _, event := range events {
		success := true
		for _, entry := range res.Failed {
			if event.ID.String() == *entry.Id {
				success = false
				break
			}
		}

		// In a production system, you would use proper metrics here
		_ = success
	}

	return nil
}

// splitToChunks splits slice into chunks of specified size
func splitToChunks[T any](slice []T, chunkSize int) [][]T {
	var chunks [][]T
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}