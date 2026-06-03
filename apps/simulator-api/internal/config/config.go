package config

import "os"

type Config struct {
	Port           string
	DatabaseURL    string
	DefaultOCPPURL string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "7070"
	}

	databaseURL := os.Getenv("DATABASE_URL")

	defaultURL := os.Getenv("DEFAULT_OCPP_URL")
	if defaultURL == "" {
		defaultURL = "ws://localhost:8080/ocpp"
	}

	return Config{
		Port:           port,
		DatabaseURL:    databaseURL,
		DefaultOCPPURL: defaultURL,
	}
}
