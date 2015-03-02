package email

import (
	"fmt"
	"log"
	"net/smtp"

	"github.com/vhakulinen/push-server/config"
	"github.com/vhakulinen/push-server/db"
)

var (
	addr     string
	username string
	password string
	host     string
	from     string
	domain   string

	configLoaded = false
)

const regMessageRaw = "This email address was used while registering to push-serv\n" +
	"To complite this regitration process, follow this link: %s\n\n" +
	"If you did not register to this service, ignore this message\n\nDo not reply to this message"

var SendRegistrationEmail = func(u *db.User) error {
	if !configLoaded {
		loadConfig()
	}
	auth := smtp.PlainAuth("", username, password, host)
	uri := fmt.Sprintf("%s/activate/?email=%s&key=%s", domain, u.Email, u.ActivateToken)
	regMessage := fmt.Sprintf(regMessageRaw, uri)
	// NOTE: This will block
	err := smtp.SendMail(addr, auth, from, []string{u.Email}, []byte(regMessage))
	if err != nil {
		log.Printf("Error while sending registration email! (%v)", err)
	}
	return err
}

func loadConfig() {
	host, _ = config.Config.String("smtp", "host")
	port, _ := config.Config.Int("smtp", "port")

	addr = fmt.Sprintf("%s:%d", host, port)
	domain, _ = config.Config.String("default", "domain")

	username, _ = config.Config.String("smtp", "username")
	password, _ = config.Config.String("smtp", "password")
	from, _ = config.Config.String("smtp", "from")

	configLoaded = true
}
