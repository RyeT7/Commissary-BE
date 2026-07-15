package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr       string
	GRPCAddr       string
	BlobDir        string
	DatabaseURL    string
	AllowedOrigin  string
	SecureCookies  bool
	MaxUploadBytes int64
}

func Default() Config {
	return Config{
		HTTPAddr:       ":8080",
		GRPCAddr:       ":9090",
		BlobDir:        "./data/blobs",
		AllowedOrigin:  "http://localhost:5173",
		SecureCookies:  false,
		MaxUploadBytes: 2 << 30,
	}
}

func FromEnv() Config {
	cfg := Default()
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("GRPC_ADDR"); v != "" {
		cfg.GRPCAddr = v
	}
	if v := os.Getenv("BLOB_DIR"); v != "" {
		cfg.BlobDir = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("ALLOWED_ORIGIN"); v != "" {
		cfg.AllowedOrigin = v
	}
	if v := os.Getenv("COOKIE_SECURE"); v == "true" || v == "1" {
		cfg.SecureCookies = true
	}
	if v := os.Getenv("MAX_UPLOAD_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.MaxUploadBytes = n
		}
	}
	return cfg
}
