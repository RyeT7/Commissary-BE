package config

import "os"

type Config struct {
	HTTPAddr    string
	GRPCAddr    string
	BlobDir     string
	DatabaseURL string
}

func Default() Config {
	return Config{
		HTTPAddr: ":8080",
		GRPCAddr: ":9090",
		BlobDir:  "./data/blobs",
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
	return cfg
}
