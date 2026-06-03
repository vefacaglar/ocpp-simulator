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

	// DBPath resolution order:
	//   1. OCPP_SIMULATOR_DB  (canonical, matches the *_DB naming
	//      convention used by ocpp-core's OCPP_CORE_DB and the
	//      docker-compose.yml volume mapping)
	//   2. DB_PATH            (legacy, kept for back-compat with
	//      older run scripts)
	//   3. ocpp-simulator.db  (relative to the working directory;
	//      useful for `go run` from the repo root)
	dbPath := os.Getenv("OCPP_SIMULATOR_DB")
	if dbPath == "" {
		dbPath = os.Getenv("DB_PATH")
	}
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
