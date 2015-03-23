package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/vhakulinen/push-server/utils"

	"crypto/sha256"

	"code.google.com/p/go-uuid/uuid"
)

const (
	MinPasswordLength   = 6
	PasswordSaltLength  = 16
	activateTokenLength = 6

	emailRegexStr = "(\\w[-._\\w]*\\w@\\w[-._\\w]*\\w\\.\\w{2,3})"
)

type User struct {
	Id         int64
	CreatedAt  time.Time
	ModifiedAt time.Time
	DeletedAt  time.Time

	Active        bool
	ActivateToken string

	Email      string `sql:"not null;unique"`
	Password   string
	Token      string `sql:"unique"`
	GCMClients []GCMClient
}

// Creates new user and saves it to database
func NewUser(email, password string) (*User, error) {
	u := new(User)
	if email == "" || password == "" {
		return nil, fmt.Errorf("Email and password required")
	} else if len(password) < MinPasswordLength {
		return nil, fmt.Errorf("Min. password length is %d", MinPasswordLength)
	}
	ok, err := regexp.Match(emailRegexStr, []byte(email))
	if err != nil {
		log.Printf("Failed to create regex string for email matching! (%v)", err)
		return nil, fmt.Errorf("Invalid email address")
	} else if ok == false {
		return nil, fmt.Errorf("Invalid email address")
	}
	if db.Where("email = ?", email).First(u).RecordNotFound() {
		u = &User{
			Email:    email,
			Password: password,
			Token:    uuid.NewRandom().String(),
		}
		if err := db.Save(u).Error; err != nil {
			log.Printf("Error in NewUser() (%v)", err)
			return nil, fmt.Errorf("Something went wrong!")
		}
		return u, nil
	}
	return nil, fmt.Errorf("User exists")
}

func (u *User) BeforeCreate() error {
	// Password hashing and salting
	if len(u.Password) < MinPasswordLength {
		return errors.New("Password is too short")
	}
	salt := utils.RandomString(PasswordSaltLength)
	b := sha256.Sum256([]byte(u.Password + salt))
	u.Password = salt + fmt.Sprintf("%x", string(b[:]))

	// Activate token
	u.ActivateToken = utils.RandomString(activateTokenLength)
	return nil
}

func (u *User) AfterFind() {
	gcmClients := []GCMClient{}
	db.Where("token = ?", u.Token).Find(&gcmClients)
	u.GCMClients = gcmClients
}

func (u *User) Activate() {
	u.Active = true
	db.Save(u)
}

func (u *User) Save() {
	db.Save(u)
}

func (u *User) ValidatePassword(password string) bool {
	// TODO: Check that slice is not out of bounds
	hash := sha256.Sum256([]byte(password + u.Password[:PasswordSaltLength]))
	if len(u.Password) > PasswordSaltLength+MinPasswordLength {
		if fmt.Sprintf("%x", hash[:]) == u.Password[PasswordSaltLength:] {
			return true
		}
	} else {
		log.Println("Invalid password in database (password length is too short)")
	}
	return false
}

type PushData struct {
	Id        int64     `json:"-"`
	CreatedAt time.Time `json:"-"`
	DeletedAt time.Time `json:"-"`

	Accessed bool

	UnixTimeStamp int64
	Title         string `sql:"not null"`
	Body          string
	Token         string `sql:"not null" json:"-"`
}

func SavePushData(title, body, token string, timestamp int64) (p *PushData, err error) {
	if timestamp < 0 {
		return nil, fmt.Errorf("Timestamp can't be less than 0")
	}
	if title == "" || token == "" {
		return nil, fmt.Errorf("token and title required")
	}

	// Check that token exists
	if db.Where("token = ?", token).First(&User{}).RecordNotFound() {
		return nil, fmt.Errorf("Token doesn't exist")
	}

	p = &PushData{
		Title:         title,
		Body:          body,
		Token:         token,
		UnixTimeStamp: timestamp,
		Accessed:      false,
	}
	if err = db.Save(p).Error; err != nil {
		fmt.Printf("%v", err)
		return nil, err
	}
	return p, nil
}

func SavePushDataMinimal(title, body, token string) (p *PushData, err error) {
	return SavePushData(title, body, token, 0)
}

func (p *PushData) SetAccessed() {
	p.Accessed = true
	p.Save()
}

func (p *PushData) Save() {
	db.Save(p)
}

func (p *PushData) Delete() {
	db.Delete(p)
}

func (p *PushData) ToJson() ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return b, nil
}

type GCMClient struct {
	Id         int64
	CreatedAt  time.Time
	ModifiedAt time.Time
	DeletedAt  time.Time

	GCMId string `sql:"not null;unique" gorm:"column:gcm_id"`
	Token string `sql:"not null"`
}

func RegisterGCMClient(gcmId, token string) (*GCMClient, error) {
	u := new(User)
	if db.Where("token = ?", token).First(u).RecordNotFound() {
		return nil, fmt.Errorf("Token not found")
	}
	g := new(GCMClient)
	if db.Where("gcm_id = ?", gcmId).First(g).RecordNotFound() {
		// If the client doesnt exist, create it
		g = &GCMClient{
			GCMId: gcmId,
			Token: token,
		}
		g.Save()
		u.GCMClients = append(u.GCMClients, *g)
		u.Save()
		return g, nil
	} else if g.Token == u.Token {
		// Same token as before, so let it be
		return nil, nil
	} else {
		// If the client has already registered, update the token
		// But before that, delete the GCMClient from the old token's client list
		oldu := new(User)
		if !db.Where("token = ?", g.Token).First(oldu).RecordNotFound() {
			var pos = -1
			for i, client := range oldu.GCMClients {
				if client.GCMId == g.GCMId {
					pos = i
					break
				}
			}
			if pos != -1 {
				oldu.GCMClients = append(oldu.GCMClients[:pos], oldu.GCMClients[pos+1:]...)
				oldu.Save()
			}
		}
		g.Token = token
		g.Save()
		u.GCMClients = append(u.GCMClients, *g)
		u.Save()
		return g, nil
	}
}

func (g GCMClient) TableName() string {
	return "gcm_clients"
}

func (g *GCMClient) Save() {
	db.Save(g)
}
