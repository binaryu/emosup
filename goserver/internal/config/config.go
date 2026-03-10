package config

import "os"

type Config struct {
	HTTPAddr string
}

func Load() Config {
	addr := os.Getenv("GO_EMOS_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	return Config{
		HTTPAddr: addr,
	}
}
