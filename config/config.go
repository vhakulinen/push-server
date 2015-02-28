package config

import (
	"log"

	"github.com/robfig/config"
)

var Config *config.Config

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
