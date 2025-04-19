package config

import "time"

// WebhookConfig contains the configuration for webhooks.
type WebhookConfig struct {
	// Url is the URL to send the webhook to. If empty, no webhook will be sent.
	Url string `yaml:"url"`
	// Authentication is the authorization header for the webhook request.
	// It can either be a Bearer token or a Basic auth string.
	Authentication string `yaml:"authentication"`
	// Timeout is the timeout for the webhook request.
	Timeout time.Duration `yaml:"timeout"`
}
