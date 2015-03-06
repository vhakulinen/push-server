package utils

import (
	"log"

	"github.com/alexjlockwood/gcm"
	"github.com/vhakulinen/push-server/config"
)

const RETRY_COUNT = 2

var gcmSender *gcm.Sender

var loaded = false

var SendGcmPing = func(regIds []string) {
	if !loaded {
		loadGcmSender()
		loaded = true
	}

	gcmData := map[string]interface{}{"message": "ping"}
	msg := gcm.NewMessage(gcmData, regIds...)

	_, err := gcmSender.Send(msg, RETRY_COUNT)
	if err != nil {
		log.Printf("Failed to send GCM message (%v)", err)
	}
}

func loadGcmSender() {
	gcmApiKey, err := config.Config.String("gcm", "ApiKey")

	if err != nil {
		log.Fatal(err)
	}

	// Create GCM sender which we'll use to send stuff to GCM servers
	gcmSender = &gcm.Sender{ApiKey: gcmApiKey}
}
