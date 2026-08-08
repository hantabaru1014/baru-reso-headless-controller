package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type EnvConfig struct {
	Database        DatabaseConfig
	Auth            AuthConfig
	Docker          DockerConfig
	GRPC            GRPCConfig
	Worker          WorkerConfig
	Server          ServerConfig
	RustFS          RustFSConfig
	ResoniteLink    ResoniteLinkConfig
	ResoniteBuild   ResoniteBuildConfig
}

type DatabaseConfig struct {
	URL string
}

type AuthConfig struct {
	JWTSecret string
}

type DockerConfig struct {
	HeadlessImageName string
	FluentdAddress    string
}

type GRPCConfig struct {
	ConnectTimeout time.Duration
	CallTimeout    time.Duration
}

type WorkerConfig struct {
	// ContentCheckInterval controls how often ContentPoller polls
	// versions.json AND the builder image AppVersion label.
	ContentCheckInterval time.Duration
	// AutoBuildNewVersions は versions.json 上で新規に検知された headless / prerelease
	// バージョンを自動ビルドキューに投入するかどうか.
	AutoBuildNewVersions bool
	// AutoBuildOnAppVersionBump は builder image の Headless/AppVersion が上がった時に、
	// 対象ブランチ (headless / prerelease) の built 済みバージョンを自動再ビルドするかどうか.
	AutoBuildOnAppVersionBump bool
	EventReconnectDelay       time.Duration
	EventMaxReconnectWait     time.Duration
	// HostEventPollInterval controls how often HostEventWatcher reconciles
	// its set of per-host event-stream subscriptions against the RUNNING
	// hosts in the DB.
	HostEventPollInterval time.Duration
	// UpgradeCheckInterval controls how often HostUpgradeOrchestrator
	// polls for newly available container image tags and reconciles its
	// drain set.
	UpgradeCheckInterval time.Duration
}

// ResoniteBuildConfig はローカルで Resonite headless container image を
// ビルドするための設定. Steam 認証情報と builder image の起動設定を持つ.
type ResoniteBuildConfig struct {
	// Steam authentication credentials.
	SteamUsername    string
	SteamPassword    string
	HeadlessPassword string

	// Steam app / depot IDs.
	AppID          string // Resonite の Steam AppID (default 2519830)
	HeadlessDepotID string // Resonite headless の Steam DepotID

	// builder image の起動設定. controller は builder image を one-shot container として
	// 起動し, ホストの docker.sock 経由で inner build を走らせる.
	BuilderImage     string // builder image 名 (tag 込み)
	DockerSocketPath string // builder container にマウントするホスト側 docker.sock パス

	// versions.json の source URL.
	VersionsJSONURL string
}

type ServerConfig struct {
	Host            string
	FrontDevMode    bool
	FrontDevURL     string
	ShutdownTimeout time.Duration
	SessionPortMin  int
	SessionPortMax  int
	// PublicIP は headless container を動かしているホストのグローバル IP (またはホスト名).
	// container に PublicIp として渡され、QUIC のセッション URL の announce に使われる.
	// 設定されている場合のみ QUIC 用のポートも自動割り当てする.
	PublicIP string
}

// ResoniteLinkConfig は ResoniteLink WebSocket ブリッジ用の設定.
type ResoniteLinkConfig struct {
	// TokenTTL は IssueResoniteLinkConnection が発行する短期トークンの有効期間.
	TokenTTL time.Duration
	// ReadyTimeout は WebSocket upgrade 前に headless container から
	// ResoniteLinkReady を受信するまでのタイムアウト.
	ReadyTimeout time.Duration
	// AllowedOrigins は WebSocket upgrade で許可する Origin パターン.
	// 空なら same-origin のみ (CheckOrigin で Host と Origin が一致するかを見る).
	// "*" を含めると全許可.
	AllowedOrigins []string
}

type RustFSConfig struct {
	Endpoint             string
	AccessKey            string
	SecretKey            string
	UseSSL               bool
	WorldDownloadsBucket string
	BlobTTLDays          int
}

func LoadEnvConfig() (*EnvConfig, error) {
	cfg := &EnvConfig{}

	cfg.Database.URL = os.Getenv("DB_URL")

	cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")

	cfg.Docker.HeadlessImageName = os.Getenv("HEADLESS_IMAGE_NAME")
	cfg.Docker.FluentdAddress = os.Getenv("CONTAINER_LOGS_FLUENTD_ADDRESS")

	cfg.GRPC.ConnectTimeout = getEnvDuration("GRPC_CONNECT_TIMEOUT", 5*time.Second)                      //nolint:mnd // default
	cfg.GRPC.CallTimeout = getEnvDuration("GRPC_CALL_TIMEOUT", 10*time.Second)                           //nolint:mnd // default

	cfg.Worker.ContentCheckInterval = getEnvDuration("CONTENT_CHECK_INTERVAL", time.Hour)
	cfg.Worker.AutoBuildNewVersions = getEnvBoolWithDefault("AUTO_BUILD_NEW_VERSIONS", true)
	cfg.Worker.AutoBuildOnAppVersionBump = getEnvBoolWithDefault("AUTO_BUILD_ON_APP_VERSION_BUMP", true)
	cfg.Worker.EventReconnectDelay = getEnvDuration("EVENT_WATCHER_RECONNECT_DELAY", 5*time.Second)      //nolint:mnd // default
	cfg.Worker.EventMaxReconnectWait = getEnvDuration("EVENT_WATCHER_MAX_RECONNECT_WAIT", 5*time.Minute) //nolint:mnd // default
	cfg.Worker.HostEventPollInterval = getEnvDuration("HOST_EVENT_POLL_INTERVAL", 10*time.Second)        //nolint:mnd // default
	cfg.Worker.UpgradeCheckInterval = getEnvDuration("UPGRADE_CHECK_INTERVAL", time.Minute)

	cfg.ResoniteBuild.SteamUsername = os.Getenv("STEAM_USERNAME")
	cfg.ResoniteBuild.SteamPassword = os.Getenv("STEAM_PASSWORD")
	cfg.ResoniteBuild.HeadlessPassword = os.Getenv("HEADLESS_PASSWORD")
	cfg.ResoniteBuild.AppID = getEnvWithDefault("RESONITE_APP_ID", "2519830")
	cfg.ResoniteBuild.HeadlessDepotID = getEnvWithDefault("RESONITE_HEADLESS_DEPOT_ID", "2519832")
	cfg.ResoniteBuild.BuilderImage = getEnvWithDefault("RESONITE_BUILDER_IMAGE", "ghcr.io/hantabaru1014/baru-reso-headless-container/builder:latest")
	cfg.ResoniteBuild.DockerSocketPath = getEnvWithDefault("RESONITE_BUILDER_DOCKER_SOCKET", "/var/run/docker.sock")
	cfg.ResoniteBuild.VersionsJSONURL = getEnvWithDefault("RESONITE_VERSIONS_JSON_URL", "https://raw.githubusercontent.com/resonite-love/resonite-version-monitor/master/data/versions.json")

	cfg.Server.Host = getEnvWithDefault("HOST", ":8014")
	cfg.Server.FrontDevMode = os.Getenv("FDEV") == "true"
	cfg.Server.FrontDevURL = getEnvWithDefault("FDEV_URL", "http://localhost:5173")
	cfg.Server.ShutdownTimeout = getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second) //nolint:mnd // default

	portMin, portMax, err := parseSessionPortEnv()
	if err != nil {
		return nil, err
	}

	cfg.Server.SessionPortMin = portMin
	cfg.Server.SessionPortMax = portMax
	cfg.Server.PublicIP = strings.TrimSpace(os.Getenv("HEADLESS_PUBLIC_IP"))

	cfg.ResoniteLink.TokenTTL = getEnvDuration("RESONITE_LINK_TOKEN_TTL", 5*time.Hour)    //nolint:mnd // default
	cfg.ResoniteLink.ReadyTimeout = getEnvDuration("RESONITE_LINK_READY_TIMEOUT", 5*time.Second) //nolint:mnd // default
	cfg.ResoniteLink.AllowedOrigins = parseCSV(os.Getenv("RESONITE_LINK_ALLOWED_ORIGINS"))

	cfg.RustFS.Endpoint = os.Getenv("RUSTFS_ENDPOINT")
	cfg.RustFS.AccessKey = os.Getenv("RUSTFS_ACCESS_KEY")
	cfg.RustFS.SecretKey = os.Getenv("RUSTFS_SECRET_KEY")
	cfg.RustFS.UseSSL = os.Getenv("RUSTFS_USE_SSL") == "true"
	cfg.RustFS.WorldDownloadsBucket = os.Getenv("WORLD_DOWNLOADS_BUCKET_NAME")
	cfg.RustFS.BlobTTLDays = getEnvInt("BLOB_TTL_DAYS", 3) //nolint:mnd // default

	return cfg, nil
}

func (c *EnvConfig) Validate() error {
	if c.Auth.JWTSecret == "" {
		return errors.New("JWT_SECRET is required")
	}

	if c.Database.URL == "" {
		return errors.New("DB_URL is required")
	}

	if c.Docker.HeadlessImageName == "" {
		return errors.New("HEADLESS_IMAGE_NAME is required")
	}

	if c.ResoniteBuild.SteamUsername == "" {
		return errors.New("STEAM_USERNAME is required")
	}

	if c.ResoniteBuild.SteamPassword == "" {
		return errors.New("STEAM_PASSWORD is required")
	}

	if c.ResoniteBuild.HeadlessPassword == "" {
		return errors.New("HEADLESS_PASSWORD is required")
	}

	if c.RustFS.Endpoint == "" {
		return errors.New("RUSTFS_ENDPOINT is required")
	}

	if c.RustFS.AccessKey == "" {
		return errors.New("RUSTFS_ACCESS_KEY is required")
	}

	if c.RustFS.SecretKey == "" {
		return errors.New("RUSTFS_SECRET_KEY is required")
	}

	if c.RustFS.WorldDownloadsBucket == "" {
		return errors.New("WORLD_DOWNLOADS_BUCKET_NAME is required")
	}

	if c.RustFS.BlobTTLDays <= 0 {
		return errors.New("BLOB_TTL_DAYS must be a positive integer")
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

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}

	return defaultValue
}

func getEnvBoolWithDefault(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}

	return v == "true" || v == "1"
}

func parseCSV(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
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
		return 0, 0, errors.New("SESSION_PORT_MIN and SESSION_PORT_MAX must both be set or both be unset")
	}

	return portMin, portMax, nil
}
