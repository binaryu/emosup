package model

type AppConfig struct {
	Server   ServerConfig   `json:"server"`
	OpenList OpenListConfig `json:"openlist"`
	Aria2    Aria2Config    `json:"aria2"`
	Emos     EmosConfig     `json:"emos"`
	Worker   WorkerConfig   `json:"worker"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type OpenListConfig struct {
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

type Aria2Config struct {
	RPCURL                string `json:"rpc_url"`
	Secret                string `json:"secret"`
	DownloadDir           string `json:"download_dir"`
	PollIntervalSeconds   int    `json:"poll_interval_seconds"`
	ConnectTimeoutSeconds int    `json:"connect_timeout_seconds"`
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
	SaveRetryIntervalSeconds int    `json:"save_retry_interval_seconds"`
	SaveRetryMaxAttempts     int    `json:"save_retry_max_attempts"`
	TMDBAPIKey               string `json:"tmdb_api_key"`
	ProxyBackends            string `json:"proxy_backends"`
}

func DefaultAppConfig() AppConfig {
	return AppConfig{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Aria2: Aria2Config{
			RPCURL:                "http://127.0.0.1:6800/jsonrpc",
			DownloadDir:           "./data/downloads",
			PollIntervalSeconds:   3,
			ConnectTimeoutSeconds: 10,
		},
		Emos: EmosConfig{
			BaseURL: "https://emos.best",
			Storage: "default",
		},
		Worker: WorkerConfig{
			PollIntervalSeconds:      5,
			MaxConcurrency:           1,
			DownloadThreads:           4,
			UploadChunkSizeMB:        8,
			SaveRetryIntervalSeconds: 25,
			SaveRetryMaxAttempts:     8,
			ProxyBackends:            "quark,夸克",
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

	if cfg.Aria2.RPCURL == "" {
		cfg.Aria2.RPCURL = defaults.Aria2.RPCURL
	}
	if cfg.Aria2.DownloadDir == "" {
		cfg.Aria2.DownloadDir = defaults.Aria2.DownloadDir
	}
	if cfg.Aria2.PollIntervalSeconds <= 0 {
		cfg.Aria2.PollIntervalSeconds = defaults.Aria2.PollIntervalSeconds
	}
	if cfg.Aria2.ConnectTimeoutSeconds <= 0 {
		cfg.Aria2.ConnectTimeoutSeconds = defaults.Aria2.ConnectTimeoutSeconds
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

	return cfg
}
