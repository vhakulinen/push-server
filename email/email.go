package email

import (
	"fmt"
	"log"
	"net/smtp"

	"github.com/sendgrid/sendgrid-go"
	"github.com/vhakulinen/push-server/config"
	"github.com/vhakulinen/push-server/db"
)

var (
	addr   string
	host   string
	from   string
	domain string

	username string
	password string

	configLoaded = false
)

const regMessageRaw = "This email address was used while registering to push-serv\n" +
	"To complite this regitration process, follow this link:\n\nhttps://%s\n\n" +
	"If you did not register to this service, ignore this message\n\nDo not reply to this message"

var sendMail func(m, email string) error

var sendSMTP = func(m, email string) error {
	auth := smtp.PlainAuth("", username, password, host)
	// NOTE: This will block
	err := smtp.SendMail(addr, auth, from, []string{email}, []byte(m))
	if err != nil {
		log.Printf("Error while sending registration email! (%v)", err)
	}
	return err
}

var sendGRID = func(m, email string) error {
	sg := sendgrid.NewSendGridClient(username, password)
	message := sendgrid.NewMail()
	message.AddTo(email)
	message.SetSubject("Push registeration")
	message.SetText(m)
	message.SetFrom(from)
	err := sg.Send(message)
	if err != nil {
		log.Printf("Error while sending registration email! (%v)", err)
	}
	return err
}

// SendRegisterationEmail sends email to u.Email with link to activate the User
var SendRegistrationEmail = func(u *db.User) error {
	if !configLoaded {
		LoadConfig()
	}
	uri := fmt.Sprintf("%s/activate/?email=%s&key=%s", domain, u.Email, u.ActivateToken)
	regMessage := fmt.Sprintf(regMessageRaw, uri)
	return sendMail(regMessage, u.Email)
}

// LoadConfig loads this package's configuration fron config.Config package
func LoadConfig() {
	emailType, _ := config.Config.String("email", "type")
	from, _ = config.Config.String("email", "from")
	domain, _ = config.Config.String("default", "domain")
	host, _ = config.Config.String("smtp", "host")
	port, _ := config.Config.Int("smtp", "port")
	addr = fmt.Sprintf("%s:%d", host, port)

	switch emailType {
	case "smtp":
		username, _ = config.Config.String("smtp", "username")
		password, _ = config.Config.String("smtp", "password")
		sendMail = sendSMTP
		break
	case "sendgrid":
		username, _ = config.Config.String("sendgrid", "username")
		password, _ = config.Config.String("sendgrid", "password")
		sendMail = sendGRID
		break
	default:
		log.Fatal("Unsupported email type!")
		break
	}

	configLoaded = true
}
