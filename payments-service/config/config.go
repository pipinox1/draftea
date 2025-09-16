package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

type Config struct {
	ServiceName string   `mapstructure:"service_name"`
	Env         string   `mapstructure:"env"`
	Port        string   `mapstructure:"port"`
	Database    Database `mapstructure:"database"`
	AWS         AWS      `mapstructure:"aws"`
}

type Database struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

type AWS struct {
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	Region          string `mapstructure:"region"`
	EndpointSNS     string `mapstructure:"endpoint_sns"`
	EndpointSQS     string `mapstructure:"endpoint_sqs"`
	SNSTopicArn     string `mapstructure:"sns_topic_arn"`
	SQSQueueURL     string `mapstructure:"sqs_queue_url"`
}

func ReadConfig() (*Config, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("unable to get current file")
	}

	configDir := filepath.Join(filepath.Dir(filename))
	viper.SetConfigName(getConfigName())
	viper.SetConfigType("json")
	viper.AddConfigPath(configDir)

	// Allow environment variables to override config
	viper.AutomaticEnv()
	viper.SetEnvPrefix("PAYMENT")

	// Set defaults from environment variables for backward compatibility
	setDefaultsFromEnv()

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

func getConfigName() string {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		return "local"
	}
	return env
}

// setDefaultsFromEnv sets defaults from environment variables for backward compatibility
func setDefaultsFromEnv() {
	// Service defaults
	viper.SetDefault("service_name", "payments-service")
	viper.SetDefault("env", getEnv("ENV", "local"))
	viper.SetDefault("port", getEnv("PORT", "8080"))

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5433)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "password")
	viper.SetDefault("database.database", "payment_system")
	viper.SetDefault("database.ssl_mode", "disable")

	// Override with DATABASE_URL if provided
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// Parse DATABASE_URL and set individual components
		// For now, just use the full URL as a fallback
		viper.Set("database.url", dbURL)
	}

	// AWS defaults
	viper.SetDefault("aws.access_key_id", getEnv("AWS_ACCESS_KEY_ID", "test"))
	viper.SetDefault("aws.secret_access_key", getEnv("AWS_SECRET_ACCESS_KEY", "test"))
	viper.SetDefault("aws.region", getEnv("AWS_DEFAULT_REGION", "us-east-1"))
	viper.SetDefault("aws.endpoint_sns", getEnv("AWS_ENDPOINT_URL_SNS", "http://localhost:4566"))
	viper.SetDefault("aws.endpoint_sqs", getEnv("AWS_ENDPOINT_URL_SQS", "http://localhost:4566"))
	viper.SetDefault("aws.sns_topic_arn", getEnv("SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:000000000000:payment-events"))
	viper.SetDefault("aws.sqs_queue_url", getEnv("SQS_QUEUE_URL", "http://localhost:4566/000000000000/payment-events"))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetDatabaseURL constructs database URL from config
func (c *Config) GetDatabaseURL() string {
	// Check if full URL is provided via DATABASE_URL
	if url := viper.GetString("database.url"); url != "" {
		return url
	}

	// Construct URL from individual components
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Database,
		c.Database.SSLMode,
	)
}