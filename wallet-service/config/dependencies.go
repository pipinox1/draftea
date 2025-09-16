package config

import (
	"context"
	"fmt"
	"log"

	sharedinfra "github.com/draftea/payment-system/shared/infrastructure"
	"github.com/draftea/payment-system/wallet-service/application"
	"github.com/draftea/payment-system/wallet-service/handlers"
	"github.com/draftea/payment-system/wallet-service/infrastructure"
	"github.com/draftea/payment-system/shared/telemetry"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Dependencies struct {
	// Database
	DB *sqlx.DB

	// Repositories
	WalletRepository      infrastructure.PostgresWalletRepository
	TransactionRepository infrastructure.PostgresTransactionRepository

	// Use Cases
	GetWallet      *application.GetWallet
	CreateMovement *application.CreateMovement
	RevertMovement *application.RevertMovement

	// HTTP Handlers
	WalletHandlers *handlers.WalletHandlers

	// Event Handlers
	WalletEventHandlers *handlers.WalletEventHandlers

	// Infrastructure
	EventPublisher  *sharedinfra.SNSPublisherAdapter
	EventSubscriber *sharedinfra.SQSSubscriberAdapter

	// Telemetry
	Telemetry         *telemetry.Telemetry
	TelemetryShutdown func()
}

func BuildDependencies(ctx context.Context, config *Config) (*Dependencies, error) {
	deps := &Dependencies{}

	// Initialize telemetry first
	if config.Telemetry.Enabled {
		telConfig := telemetry.WalletServiceConfig.WithOTLPEndpoint(config.Telemetry.OTLPEndpoint)
		tel, telemetryShutdown, err := telemetry.InitTelemetry(ctx, telConfig)
		if err != nil {
			log.Printf("Failed to initialize telemetry: %v", err)
			// Continue without telemetry rather than failing
		} else {
			deps.Telemetry = tel
			deps.TelemetryShutdown = telemetryShutdown
		}
	}

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
	deps.WalletRepository = *infrastructure.NewPostgresWalletRepository(db)
	deps.TransactionRepository = *infrastructure.NewPostgresTransactionRepository(db)

	// Initialize use cases
	deps.GetWallet = application.NewGetWallet(&deps.WalletRepository)
	deps.CreateMovement = application.NewCreateMovement(&deps.WalletRepository, &deps.TransactionRepository, eventPublisher)
	deps.RevertMovement = application.NewRevertMovement(&deps.WalletRepository, &deps.TransactionRepository, eventPublisher)

	// Initialize handlers
	deps.WalletHandlers = handlers.NewWalletHandlers(deps.GetWallet, deps.CreateMovement, deps.RevertMovement)
	deps.WalletEventHandlers = handlers.NewWalletEventHandlers(deps.CreateMovement, deps.RevertMovement)

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

	if d.TelemetryShutdown != nil {
		d.TelemetryShutdown()
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing dependencies: %v", errs)
	}

	return nil
}