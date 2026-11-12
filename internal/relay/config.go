// Package relay implements consumer-owned durable delivery of chat references.
// It has no broker credentials, approval capability, shell runner, or model client.
package relay

import (
	"net"
	"net/url"
	"path/filepath"
	"regexp"

	"github.com/angelmsger/wecom-chat-cli/internal/securefs"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
	"github.com/angelmsger/wecom-chat-cli/pkg/safejson"
)

type Config struct {
	Version            int    `json:"version"`
	URL                string `json:"webhook_url"`
	TokenEnv           string `json:"token_env,omitempty"`
	BatchWindowSeconds int    `json:"batch_window_seconds"`
	BatchSize          int    `json:"batch_size"`
	QueueLimit         int    `json:"queue_limit"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	RetryLimit         int    `json:"retry_limit"`
	RetryBaseSeconds   int    `json:"retry_base_seconds"`
	DeliveryMode       string `json:"delivery_mode,omitempty"`
	MinDeliverySeconds int    `json:"min_delivery_interval_seconds,omitempty"`
}

const ExplicitAck = "explicit_ack"
const HTTPAccepted = "http_accepted"

func (c Config) Mode() string {
	if c.DeliveryMode == "" {
		return ExplicitAck
	}
	return c.DeliveryMode
}

func DefaultConfig() Config {
	return Config{Version: 1, BatchWindowSeconds: 1, BatchSize: 50, QueueLimit: 1000, TimeoutSeconds: 10, RetryBaseSeconds: 5}
}

func LoadConfig(path string) (Config, error) {
	c := DefaultConfig()
	abs, err := filepath.Abs(path)
	if err != nil || path == "" {
		return c, configError()
	}
	b, err := securefs.ReadPrivate(abs, 16384, true)
	if err != nil || safejson.Decode(b, &c, true) != nil {
		return c, configError()
	}
	return c, c.Validate()
}

func configError() error {
	return e.New(e.CategoryConfig, "RELAY_CONFIG", "Use a protected relay JSON configuration with a fixed HTTPS webhook (HTTP only for literal loopback), bounded delivery limits.").WithNextSteps("wecom-chat-cli relay run --help", "Review deploy/relay.example.json and docs/host-integration.md.")
}

func (c Config) Validate() error {
	if (c.Mode() != ExplicitAck && c.Mode() != HTTPAccepted) || c.MinDeliverySeconds < 0 || c.MinDeliverySeconds > 3600 || (c.Mode() == HTTPAccepted && c.TokenEnv == "") {
		return configError()
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.ForceQuery || len(c.URL) > 2048 {
		return configError()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && net.ParseIP(u.Hostname()).IsLoopback()) {
		return configError()
	}
	if c.Version != 1 || c.BatchWindowSeconds < 1 || c.BatchWindowSeconds > 300 || c.BatchSize < 1 || c.BatchSize > 100 || c.QueueLimit < c.BatchSize || c.QueueLimit > 10000 || c.TimeoutSeconds < 1 || c.TimeoutSeconds > 30 || c.RetryLimit < 0 || c.RetryLimit > 20 || c.RetryBaseSeconds < 1 || c.RetryBaseSeconds > 300 {
		return configError()
	}
	if c.TokenEnv != "" && !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`).MatchString(c.TokenEnv) {
		return configError()
	}
	return nil
}
