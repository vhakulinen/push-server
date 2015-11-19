package config

import (
	"log"

	"github.com/robfig/config"
)

// Config is the global configuration object of push-server.
// Use GetConfig function to initialize this object.
var Config *config.Config

// GetConfig initializes global Config object specified in this package
func GetConfig(path string) *config.Config {
	if Config != nil {
		return Config
	}

	var err error
	Config, err = config.ReadDefault(path)
	if err != nil {
		log.Fatalf("Failed to read config file! (%v)", err)
	}

	return Config
}
