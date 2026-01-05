package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type EnvConfig struct {
	Database DatabaseConfig
	Auth     AuthConfig
	Docker   DockerConfig
	GRPC     GRPCConfig
	Worker   WorkerConfig
	Server   ServerConfig
}

type DatabaseConfig struct {
	URL string
}

type AuthConfig struct {
	JWTSecret string
}

type DockerConfig struct {
	HeadlessImageName    string
	FluentdAddress       string
	GHCRAuthToken        string
	HeadlessRegistryAuth string
}

type GRPCConfig struct {
	ConnectTimeout time.Duration
	CallTimeout    time.Duration
}

type WorkerConfig struct {
	ImageCheckInterval    time.Duration
	AutoPullNewImage      bool
	EventReconnectDelay   time.Duration
	EventMaxReconnectWait time.Duration
}

type ServerConfig struct {
	Host            string
	FrontDevMode    bool
	FrontDevURL     string
	ShutdownTimeout time.Duration
	SessionPortMin  int
	SessionPortMax  int
}

func LoadEnvConfig() (*EnvConfig, error) {
	cfg := &EnvConfig{}

	cfg.Database.URL = os.Getenv("DB_URL")

	cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")

	cfg.Docker.HeadlessImageName = os.Getenv("HEADLESS_IMAGE_NAME")
	cfg.Docker.FluentdAddress = os.Getenv("CONTAINER_LOGS_FLUENTD_ADDRESS")
	cfg.Docker.GHCRAuthToken = os.Getenv("GHCR_AUTH_TOKEN")
	cfg.Docker.HeadlessRegistryAuth = os.Getenv("HEADLESS_REGISTRY_AUTH")

	cfg.GRPC.ConnectTimeout = getEnvDuration("GRPC_CONNECT_TIMEOUT", 5*time.Second)
	cfg.GRPC.CallTimeout = getEnvDuration("GRPC_CALL_TIMEOUT", 10*time.Second)

	cfg.Worker.ImageCheckInterval = getEnvDurationSec("IMAGE_CHECK_INTERVAL_SEC", 15*time.Second)
	cfg.Worker.AutoPullNewImage = os.Getenv("AUTO_PULL_NEW_IMAGE") == "true"
	cfg.Worker.EventReconnectDelay = getEnvDuration("EVENT_WATCHER_RECONNECT_DELAY", 5*time.Second)
	cfg.Worker.EventMaxReconnectWait = getEnvDuration("EVENT_WATCHER_MAX_RECONNECT_WAIT", 5*time.Minute)

	cfg.Server.Host = getEnvWithDefault("HOST", ":8014")
	cfg.Server.FrontDevMode = os.Getenv("FDEV") == "true"
	cfg.Server.FrontDevURL = getEnvWithDefault("FDEV_URL", "http://localhost:5173")
	cfg.Server.ShutdownTimeout = getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second)

	portMin, portMax, err := parseSessionPortEnv()
	if err != nil {
		return nil, err
	}
	cfg.Server.SessionPortMin = portMin
	cfg.Server.SessionPortMax = portMax

	return cfg, nil
}

func (c *EnvConfig) Validate() error {
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.Database.URL == "" {
		return fmt.Errorf("DB_URL is required")
	}
	if c.Docker.HeadlessImageName == "" {
		return fmt.Errorf("HEADLESS_IMAGE_NAME is required")
	}

	return nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}

func getEnvDurationSec(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}

func parseSessionPortEnv() (int, int, error) {
	portMin, portMax := 0, 0
	portMinStr := os.Getenv("SESSION_PORT_MIN")
	portMaxStr := os.Getenv("SESSION_PORT_MAX")

	if portMinStr != "" && portMaxStr != "" {
		var err error
		portMin, err = strconv.Atoi(portMinStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid SESSION_PORT_MIN: %s", portMinStr)
		}
		portMax, err = strconv.Atoi(portMaxStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid SESSION_PORT_MAX: %s", portMaxStr)
		}
		if portMin < 1024 || portMin > 65535 {
			return 0, 0, fmt.Errorf("SESSION_PORT_MIN(%d) must be between 1024 and 65535", portMin)
		}
		if portMax < 1 || portMax > 65535 {
			return 0, 0, fmt.Errorf("SESSION_PORT_MAX(%d) must be between 1 and 65535", portMax)
		}
		if portMin > portMax {
			return 0, 0, fmt.Errorf("invalid port range: SESSION_PORT_MIN(%d) > SESSION_PORT_MAX(%d)", portMin, portMax)
		}
	} else if (portMinStr != "" && portMaxStr == "") || (portMinStr == "" && portMaxStr != "") {
		return 0, 0, fmt.Errorf("SESSION_PORT_MIN and SESSION_PORT_MAX must both be set or both be unset")
	}

	return portMin, portMax, nil
}
