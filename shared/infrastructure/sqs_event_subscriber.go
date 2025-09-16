package infrastructure

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/draftea/payment-system/shared/events"
	"github.com/pkg/errors"
)

const (
	SQSMessageIDKey     = "sqs_message_id"
	SQSReceiptHandleKey = "sqs_receipt_handle"
)

type sqsMessage struct {
	Message types.Message
	Event   *events.Event
	Err     error
}

// EventHandler wraps the Event Handler interface
type EventHandler interface {
	HandlerID() string
	Handle(ctx context.Context, event *events.Event) error
}

// EventHandlerFunc creates a handler from a function
type EventHandlerFunc struct {
	id string
	fn func(ctx context.Context, event *events.Event) error
}

func NewEventHandlerFunc(id string, fn func(ctx context.Context, event *events.Event) error) *EventHandlerFunc {
	return &EventHandlerFunc{
		id: id,
		fn: fn,
	}
}

func (h *EventHandlerFunc) HandlerID() string {
	return h.id
}

func (h *EventHandlerFunc) Handle(ctx context.Context, event *events.Event) error {
	return h.fn(ctx, event)
}

// SQSEventSubscriber implements event subscription using AWS SQS
type SQSEventSubscriber struct {
	mux              sync.RWMutex
	inboundMessages  chan *sqsMessage
	outboundMessages chan *sqsMessage
	cancel           context.CancelFunc
	running          atomic.Bool
	options          *sqsSubscriberOptions

	client   *sqs.Client
	queueURL string
	handler  EventHandler
}

type sqsSubscriberOptions struct {
	name                           string
	workers                        int32
	readers                        int32
	cleaners                       int32
	maxNumberOfMessages            int32
	waitTimeSeconds                int32
	visibilityTimeout              int32
	sleepTimeAfterEmptyReceive     time.Duration
	sleepTimeAfterError            time.Duration
	ack                            bool
	extendVisibilityTimeoutOnError bool
	receiveCountRange              int32
	visibilityTimeoutOffset        int32
	maxVisibilityTimeout           int32
}

type SQSSubscriberOption func(*sqsSubscriberOptions)

func WithWorkers(workers int32) SQSSubscriberOption {
	return func(o *sqsSubscriberOptions) {
		o.workers = workers
	}
}

func WithReaders(readers int32) SQSSubscriberOption {
	return func(o *sqsSubscriberOptions) {
		o.readers = readers
	}
}

func WithVisibilityTimeout(timeout int32) SQSSubscriberOption {
	return func(o *sqsSubscriberOptions) {
		o.visibilityTimeout = timeout
	}
}

// NewSQSEventSubscriber creates a new SQS event subscriber
func NewSQSEventSubscriber(
	client *sqs.Client,
	queueURL string,
	handler EventHandler,
	opts ...SQSSubscriberOption,
) *SQSEventSubscriber {
	options := &sqsSubscriberOptions{
		name:                           "sqs",
		workers:                        30,
		readers:                        1,
		cleaners:                       2,
		maxNumberOfMessages:            5,
		waitTimeSeconds:                15,
		visibilityTimeout:              30,
		sleepTimeAfterEmptyReceive:     10 * time.Second,
		sleepTimeAfterError:            20 * time.Second,
		ack:                            true,
		extendVisibilityTimeoutOnError: true,
		receiveCountRange:              3,
		visibilityTimeoutOffset:        30,
		maxVisibilityTimeout:           900, // 15 minutes
	}

	for _, opt := range opts {
		opt(options)
	}

	return &SQSEventSubscriber{
		client:           client,
		queueURL:         queueURL,
		handler:          handler,
		inboundMessages:  make(chan *sqsMessage, 10),
		outboundMessages: make(chan *sqsMessage, 10),
		options:          options,
	}
}

// Start starts the SQS subscriber
func (s *SQSEventSubscriber) Start(ctx context.Context) error {
	if s.running.Load() {
		return nil
	}

	s.mux.Lock()
	defer s.mux.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	if s.inboundMessages != nil {
		close(s.inboundMessages)
	}

	if s.outboundMessages != nil {
		close(s.outboundMessages)
	}

	ctx, cancel := context.WithCancel(ctx)
	s.inboundMessages = make(chan *sqsMessage, 10)
	s.outboundMessages = make(chan *sqsMessage, 10)
	s.cancel = cancel

	for i := 0; i < int(s.options.workers); i++ {
		go s.startWorker(ctx)
	}

	for i := 0; i < int(s.options.readers); i++ {
		go s.startReader(ctx)
	}

	for i := 0; i < int(s.options.cleaners); i++ {
		go s.startCleaner(ctx)
	}

	s.running.Store(true)

	return nil
}

// Stop stops the SQS subscriber
func (s *SQSEventSubscriber) Stop(ctx context.Context) error {
	if !s.running.Load() {
		return nil
	}

	s.mux.Lock()
	defer s.mux.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	if s.inboundMessages != nil {
		close(s.inboundMessages)
	}

	if s.outboundMessages != nil {
		close(s.outboundMessages)
	}

	s.cancel = nil
	s.inboundMessages = nil
	s.outboundMessages = nil

	s.running.Store(false)

	return nil
}

func (s *SQSEventSubscriber) startWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-s.inboundMessages:
			if message == nil {
				continue
			}
			s.handle(ctx, message)
		}
	}
}

func (s *SQSEventSubscriber) startReader(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := s.read(ctx); err != nil {
				time.Sleep(s.options.sleepTimeAfterError)
			}
		}
	}
}

func (s *SQSEventSubscriber) startCleaner(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-s.outboundMessages:
			if message == nil {
				continue
			}
			if err := s.clean(ctx, message); err != nil {
				// Log error in production
				continue
			}
		}
	}
}

func (s *SQSEventSubscriber) read(ctx context.Context) error {
	output, err := s.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.queueURL),
		MaxNumberOfMessages: s.options.maxNumberOfMessages,
		WaitTimeSeconds:     s.options.waitTimeSeconds,
		VisibilityTimeout:   s.options.visibilityTimeout,
		AttributeNames: []types.QueueAttributeName{
			"ApproximateReceiveCount",
			"ApproximateFirstReceiveTimestamp",
		},
		MessageAttributeNames: []string{"All"},
	})
	if err != nil {
		return errors.Wrap(err, "failed to receive message from SQS")
	}

	if len(output.Messages) == 0 {
		time.Sleep(s.options.sleepTimeAfterEmptyReceive)
		return nil
	}

	for _, message := range output.Messages {
		var event *events.Event
		if err := json.Unmarshal([]byte(*message.Body), &event); err != nil {
			continue // Skip malformed messages
		}

		if event.Metadata == nil {
			event.Metadata = make(events.Metadata)
		}

		event.Metadata.Set(SQSMessageIDKey, *message.MessageId)
		if message.ReceiptHandle != nil {
			event.Metadata.Set(SQSReceiptHandleKey, *message.ReceiptHandle)
		}

		for k, v := range message.MessageAttributes {
			if v.StringValue != nil {
				event.Metadata.Set(k, *v.StringValue)
			}
		}

		select {
		case s.inboundMessages <- &sqsMessage{
			Message: message,
			Event:   event,
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (s *SQSEventSubscriber) handle(ctx context.Context, message *sqsMessage) {
	s.mux.RLock()
	handler := s.handler
	s.mux.RUnlock()

	if handler == nil {
		message.Err = errors.New("no handler configured")
	} else {
		message.Err = handler.Handle(ctx, message.Event)
	}

	select {
	case s.outboundMessages <- message:
	case <-ctx.Done():
	}
}

func (s *SQSEventSubscriber) clean(ctx context.Context, message *sqsMessage) error {
	if message.Err != nil {
		if s.options.extendVisibilityTimeoutOnError {
			receiveCount, err := strconv.Atoi(message.Message.Attributes["ApproximateReceiveCount"])
			if err != nil {
				receiveCount = 1
			}

			visibilityTimeout := s.options.visibilityTimeout
			visibilityTimeout += (int32(receiveCount) / s.options.receiveCountRange) * s.options.visibilityTimeoutOffset

			if visibilityTimeout > s.options.maxVisibilityTimeout {
				visibilityTimeout = s.options.maxVisibilityTimeout
			}

			_, err = s.client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
				QueueUrl:          &s.queueURL,
				ReceiptHandle:     message.Message.ReceiptHandle,
				VisibilityTimeout: visibilityTimeout,
			})
			if err != nil {
				return errors.Wrap(err, "failed to extend visibility timeout")
			}
		}
		return nil
	}

	if s.options.ack {
		_, err := s.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      &s.queueURL,
			ReceiptHandle: message.Message.ReceiptHandle,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete message from SQS")
		}
	}

	return nil
}