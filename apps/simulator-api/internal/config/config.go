package config

import "os"

type Config struct {
	Port           string
	DBPath         string
	DefaultOCPPURL string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "7070"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "ocpp-simulator.db"
	}

	defaultURL := os.Getenv("DEFAULT_OCPP_URL")
	if defaultURL == "" {
		defaultURL = "ws://localhost:8080/ocpp"
	}

	return Config{
		Port:           port,
		DBPath:         dbPath,
		DefaultOCPPURL: defaultURL,
	}
}
