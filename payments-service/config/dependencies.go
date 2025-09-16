package config

import (
	"fmt"

	"github.com/draftea/payment-system/payments-service/application"
	"github.com/draftea/payment-system/payments-service/handlers"
	"github.com/draftea/payment-system/payments-service/infrastructure"
	sharedinfra "github.com/draftea/payment-system/shared/infrastructure"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Dependencies struct {
	// Database
	DB *sqlx.DB

	// Repositories
	PaymentRepository infrastructure.PostgresPaymentRepository

	// Use Cases
	CreatePayment                       *application.CreatePaymentChoreography
	GetPayment                          *application.GetPayment
	ProcessPaymentMethod                *application.ProcessPaymentMethod
	ProcessWalletDebit                  *application.ProcessWalletDebit
	HandleExternalWebhooks              *application.HandleExternalWebhooks
	ProcessExternalProviderUpdates      *application.ProcessExternalProviderUpdates
	ProcessPaymentOperationResult       *application.ProcessPaymentOperationResult
	ProcessPaymentInconsistentOperation *application.ProcessPaymentInconsistentOperation
	RefundPayment                       *application.RefundPayment
	ProcessRefund                       *application.ProcessRefund

	// HTTP Handlers
	PaymentHandlers *handlers.PaymentHandlers

	// Event Handlers
	PaymentEventHandlers *handlers.PaymentEventHandlers

	// Infrastructure
	EventPublisher  *sharedinfra.SNSPublisherAdapter
	EventSubscriber *sharedinfra.SQSSubscriberAdapter
}

func BuildDependencies(config *Config) (*Dependencies, error) {
	deps := &Dependencies{}

	// Initialize database
	db, err := sqlx.Connect("postgres", config.GetDatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	deps.DB = db

	// Initialize AWS infrastructure
	eventPublisher, err := sharedinfra.NewSNSPublisherAdapter(config.AWS.SNSTopicArn)
	if err != nil {
		return nil, fmt.Errorf("failed to create SNS publisher: %w", err)
	}
	deps.EventPublisher = eventPublisher

	eventSubscriber, err := sharedinfra.NewSQSSubscriberAdapter(config.AWS.SQSQueueURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQS subscriber: %w", err)
	}
	deps.EventSubscriber = eventSubscriber

	// Initialize repositories
	deps.PaymentRepository = *infrastructure.NewPostgresPaymentRepository(db)

	// Initialize use cases
	deps.CreatePayment = application.NewCreatePaymentChoreography(&deps.PaymentRepository, eventPublisher)
	deps.GetPayment = application.NewGetPayment(&deps.PaymentRepository)
	deps.ProcessPaymentMethod = application.NewProcessPaymentMethod(&deps.PaymentRepository, eventPublisher)
	deps.ProcessWalletDebit = application.NewProcessWalletDebit(&deps.PaymentRepository, eventPublisher)
	deps.HandleExternalWebhooks = application.NewHandleExternalWebhooks(eventPublisher)
	deps.ProcessExternalProviderUpdates = application.NewProcessExternalProviderUpdates(&deps.PaymentRepository, eventPublisher)
	deps.ProcessPaymentOperationResult = application.NewProcessPaymentOperationResult(&deps.PaymentRepository, eventPublisher)
	deps.ProcessPaymentInconsistentOperation = application.NewProcessPaymentInconsistentOperation(&deps.PaymentRepository, eventPublisher)
	deps.RefundPayment = application.NewRefundPayment(&deps.PaymentRepository, eventPublisher)
	deps.ProcessRefund = application.NewProcessRefund(&deps.PaymentRepository, eventPublisher)

	// Initialize handlers
	deps.PaymentHandlers = handlers.NewPaymentHandlers(deps.CreatePayment, deps.GetPayment)
	deps.PaymentEventHandlers = handlers.NewPaymentEventHandlers(
		deps.ProcessPaymentMethod,
		deps.ProcessWalletDebit,
		deps.HandleExternalWebhooks,
		deps.ProcessExternalProviderUpdates,
		deps.ProcessPaymentOperationResult,
		deps.ProcessPaymentInconsistentOperation,
		deps.RefundPayment,
		deps.ProcessRefund,
	)

	return deps, nil
}

// Close closes all dependencies
func (d *Dependencies) Close() error {
	var errs []error

	if d.DB != nil {
		if err := d.DB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close database: %w", err))
		}
	}

	if d.EventPublisher != nil {
		if err := d.EventPublisher.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close event publisher: %w", err))
		}
	}

	if d.EventSubscriber != nil {
		if err := d.EventSubscriber.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close event subscriber: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing dependencies: %v", errs)
	}

	return nil
}