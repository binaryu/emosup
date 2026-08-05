package model

import "strings"

type AppConfig struct {
	Server   ServerConfig   `json:"server"`
	Auth     AuthConfig     `json:"auth"`
	Local    LocalConfig    `json:"local"`
	OpenList OpenListConfig `json:"openlist"`
	Download DownloadConfig `json:"download"`
	Emos     EmosConfig     `json:"emos"`
	Worker   WorkerConfig   `json:"worker"`
}

// LocalConfig controls on-disk media browsing (binary/Docker local source).
type LocalConfig struct {
	// Root is the absolute (or relative-to-data) path shown in「本地媒体」.
	// Empty = fall back to download_dir / data/downloads.
	Root string `json:"root"`
}

type AuthConfig struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	JWTSecret     string `json:"jwt_secret"`
	TokenTTLHours int    `json:"token_ttl_hours"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	// WebTitle is the site title shown in the browser tab and sidebar.
	WebTitle string `json:"web_title"`
}

type OpenListConfig struct {
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

// DownloadConfig controls the local download cache directory.
type DownloadConfig struct {
	// Dir is where downloaded files are cached before upload.
	Dir string `json:"dir"`
}

type EmosConfig struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
	Storage string `json:"storage"`
}

type WorkerConfig struct {
	PollIntervalSeconds      int    `json:"poll_interval_seconds"`
	MaxConcurrency           int    `json:"max_concurrency"`
	DownloadThreads          int    `json:"download_threads"`
	UploadChunkSizeMB        int    `json:"upload_chunk_size_mb"`
	UploadPartConcurrency    int    `json:"upload_part_concurrency"`
	SaveRetryIntervalSeconds int    `json:"save_retry_interval_seconds"`
	SaveRetryMaxAttempts     int    `json:"save_retry_max_attempts"`
	TMDBAPIKey               string `json:"tmdb_api_key"`
	ProxyBackends            string `json:"proxy_backends"`
	// AutoTune adapts concurrency/threads/chunk size to measured bandwidth
	// and free disk space (tuned values act as floors above the fixed ones).
	// nil means enabled (default) so pre-existing configs opt in automatically.
	AutoTune *bool `json:"auto_tune"`
}

func DefaultAppConfig() AppConfig {
	trueVal := true
	return AppConfig{
		Server: ServerConfig{
			// 0.0.0.0 so binary/systemd installs are reachable on LAN/WAN by default.
			Host:     "0.0.0.0",
			Port:     8080,
			WebTitle: "Emos Upload Panel",
		},
		Auth: AuthConfig{
			Username:      "admin",
			Password:      "admin",
			TokenTTLHours: 72,
		},
		Download: DownloadConfig{
			Dir: "./data/downloads",
		},
		Emos: EmosConfig{
			BaseURL: "https://emos.best",
			Storage: "default",
		},
		Worker: WorkerConfig{
			PollIntervalSeconds:      5,
			MaxConcurrency:           1,
			DownloadThreads:          1,
			UploadChunkSizeMB:        8,
			UploadPartConcurrency:    3,
			SaveRetryIntervalSeconds: 25,
			SaveRetryMaxAttempts:     8,
			ProxyBackends:            "quark,夸克",
			AutoTune:                 &trueVal,
		},
	}
}

func NormalizeAppConfig(cfg AppConfig) AppConfig {
	defaults := DefaultAppConfig()

	if cfg.Server.Host == "" {
		cfg.Server.Host = defaults.Server.Host
	}
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = defaults.Server.Port
	}
	if strings.TrimSpace(cfg.Server.WebTitle) == "" {
		cfg.Server.WebTitle = defaults.Server.WebTitle
	}

	if cfg.Auth.Username == "" {
		cfg.Auth.Username = defaults.Auth.Username
	}
	if cfg.Auth.Password == "" {
		cfg.Auth.Password = defaults.Auth.Password
	}
	if cfg.Auth.TokenTTLHours <= 0 {
		cfg.Auth.TokenTTLHours = defaults.Auth.TokenTTLHours
	}

	if cfg.Download.Dir == "" {
		cfg.Download.Dir = defaults.Download.Dir
	}

	if cfg.Emos.BaseURL == "" {
		cfg.Emos.BaseURL = defaults.Emos.BaseURL
	}
	if cfg.Emos.Storage == "" {
		cfg.Emos.Storage = defaults.Emos.Storage
	}

	if cfg.Worker.PollIntervalSeconds <= 0 {
		cfg.Worker.PollIntervalSeconds = defaults.Worker.PollIntervalSeconds
	}
	if cfg.Worker.UploadChunkSizeMB <= 0 {
		cfg.Worker.UploadChunkSizeMB = defaults.Worker.UploadChunkSizeMB
	}
	if cfg.Worker.SaveRetryIntervalSeconds <= 0 {
		cfg.Worker.SaveRetryIntervalSeconds = defaults.Worker.SaveRetryIntervalSeconds
	}
	if cfg.Worker.SaveRetryMaxAttempts <= 0 {
		cfg.Worker.SaveRetryMaxAttempts = defaults.Worker.SaveRetryMaxAttempts
	}
	if cfg.Worker.MaxConcurrency <= 0 {
		cfg.Worker.MaxConcurrency = defaults.Worker.MaxConcurrency
	}
	if cfg.Worker.DownloadThreads <= 0 {
		cfg.Worker.DownloadThreads = defaults.Worker.DownloadThreads
	}
	if cfg.Worker.UploadPartConcurrency <= 0 {
		cfg.Worker.UploadPartConcurrency = defaults.Worker.UploadPartConcurrency
	}
	if cfg.Worker.UploadPartConcurrency > 10 {
		cfg.Worker.UploadPartConcurrency = 10
	}

	return cfg
}
