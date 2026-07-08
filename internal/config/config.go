package config

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
