package utils

import (
	"log"

	"github.com/alexjlockwood/gcm"
	"github.com/vhakulinen/push-server/config"
)

const retryCount = 2

var gcmSender *gcm.Sender

var loaded = false

// SendGcmPing sends ping message to GCM client to notify it to pool data
var SendGcmPing = func(regIds []string) {
	if !loaded {
		LoadConfig()
		loaded = true
	}

	gcmData := map[string]interface{}{"message": "ping"}
	msg := gcm.NewMessage(gcmData, regIds...)
	msg.CollapseKey = "ping"
	msg.DelayWhileIdle = false

	_, err := gcmSender.Send(msg, retryCount)
	if err != nil {
		log.Printf("Failed to send GCM message (%v)", err)
	}
}

// LoadConfig loads this package configuration from global config.Config object
func LoadConfig() {
	gcmAPIKey, err := config.Config.String("gcm", "ApiKey")

	if err != nil {
		log.Fatal(err)
	}

	// Create GCM sender which we'll use to send stuff to GCM servers
	gcmSender = &gcm.Sender{ApiKey: gcmAPIKey}
}
