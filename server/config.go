package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	BrokerURL   string
	ClientID    string
	Username    string
	Password    string
	TopicPrefix string
	UseTLS      bool
	CAFile      string
	ExecKey     string
}

func LoadConfig() (*Config, error) {
	// Try to load .env file (ignore error if not exists)
	_ = godotenv.Load()

	config := &Config{
		BrokerURL:   getEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		ClientID:    getEnv("MQTT_CLIENT_ID", ""),
		Username:    getEnv("MQTT_USERNAME", ""),
		Password:    getEnv("MQTT_PASSWORD", ""),
		TopicPrefix: getEnv("MQTT_TOPIC_PREFIX", "mqtt-shell"),
		UseTLS:      getEnv("MQTT_USE_TLS", "false") == "true",
		CAFile:      getEnv("MQTT_CA_FILE", ""),
		ExecKey:     getEnv("EXEC_KEY", ""),
	}

	// Generate client ID if not provided
	if config.ClientID == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "server"
		}
		config.ClientID = fmt.Sprintf("mqtt-shell-server-%s", hostname)
	}

	return config, nil
}

func (c *Config) GetTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if c.CAFile != "" {
		caCert, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %v", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return strings.TrimSpace(value)
	}
	return defaultValue
}
