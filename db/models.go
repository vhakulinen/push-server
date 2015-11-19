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

	"github.com/pborman/uuid"
)

const (
	// MinPasswordLength specifies the minimiun password length
	MinPasswordLength = 6
	// PasswordSaltLength specifies the length of salt used with hashing passwords
	PasswordSaltLength  = 16
	activateTokenLength = 6

	emailRegexStr = "(\\w[-._\\w]*\\w@\\w[-._\\w]*\\w\\.\\w{2,3})"
)

// User is the user object mapped in database. Contains all relevant information about user.
type User struct {
	// ID is the primary key used in databse
	ID int64
	// CreatedAt is the date when this user was created in database level
	CreatedAt time.Time
	// ModifiedAt is the date when this user was last modified in database level
	ModifiedAt time.Time
	// DeletedAt is the date when user was /soft/ deleted in database level
	DeletedAt time.Time

	// Active is the flag indicating if the user is activated with email or not.
	// If user is not activated it cannot be used. Email activation can be skipped
	// within the server's configuration file
	Active bool
	// ActivateToken is used to securely activate the user with email. It is added
	// to the link sent in the activation email
	ActivateToken string

	// Email is the email user provoided when he/she registered to this service
	Email string `sql:"not null;unique"`
	// Password is the user's password
	Password string
	// Token is the token which is used to push/pool data
	Token string `sql:"unique"`
	// GCMClients are the clients registered with GoogleCloudMessaging service to this user
	GCMClients []GCMClient
}

// NewUser creates new user and saves it to database
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

// BeforeCreate is function ran by gorm library before the user is created.
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

// AfterFind is function rab by gorm library after database query is ran
// agains User table. In this case its used to load data to User object.
func (u *User) AfterFind() {
	gcmClients := []GCMClient{}
	db.Where("token = ?", u.Token).Find(&gcmClients)
	u.GCMClients = gcmClients
}

// Activate activates the user (sets User.Active to true and saves it to database)
func (u *User) Activate() {
	u.Active = true
	db.Save(u)
}

// Save is shortcut to save object to database
func (u *User) Save() {
	db.Save(u)
}

// ValidatePassword checks if specified password is the correct password for the user
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

// PushData is the object mapped on database. This is the object containing
// the data user may push through to other devices using this service.
type PushData struct {
	// ID is the primary key used in databse
	ID int64 `json:"-"`
	// CreatedAt is the date when this user was created in database level
	CreatedAt time.Time `json:"-"`
	// DeletedAt is the date when user was /soft/ deleted in database level
	DeletedAt time.Time `json:"-"`

	// Accessed indicates if this data has already pooled by client (the one user design flaw lies in here)
	Accessed bool `json:"-"`

	// UinxTimeStamp is the timestamp which client can specify when sending data
	// Timestamp defaults to 0 if invalid
	UnixTimeStamp int64
	Title         string `sql:"not null"`
	Body          string
	Token         string `sql:"not null" json:"-"`
	// URL to open on client side
	//
	// URL is not validated
	URL string
	// Priority defines whether we send the data to all clients, do we make seound etc.
	//
	// Possible values:
	// 1*: Send to all clients
	// 2: Don't make sound on GCM clients if TCP client is listening
	// 3: Don't send to TCP client
	// * = default
	//
	// Invalid value defaults to 1
	Priority int64 `json:"-"`
	Sound    bool
}

// SavePushData saves push data to the database
func SavePushData(title, body, token, strurl string, timestamp, priority int64) (p *PushData, err error) {
	if timestamp < 0 {
		timestamp = 0
	}
	if title == "" || token == "" {
		return nil, fmt.Errorf("token and title required")
	}
	if priority > 3 || priority < 1 {
		priority = 1
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
		Priority:      priority,
		Sound:         true,
		URL:           strurl,
	}
	if err = db.Save(p).Error; err != nil {
		fmt.Printf("%v", err)
		return nil, err
	}
	return p, nil
}

// SetAccessed sets Accessed property to true and saves it to database
func (p *PushData) SetAccessed() {
	p.Accessed = true
	p.Save()
}

// Save is shortcut to save data to database
func (p *PushData) Save() {
	db.Save(p)
}

// Delete is shortcut to delete data from database
func (p *PushData) Delete() {
	db.Delete(p)
}

// ToJSON returns this object as JSON string (byte array)
func (p *PushData) ToJSON() ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GCMClient is object mapped in database. Holds data of GoogleCloudMessaging clients
// registered by user
type GCMClient struct {
	ID int64

	GCMId string `sql:"not null;unique" gorm:"column:gcm_id"`
	Token string `sql:"not null"`
}

// RegisterGCMClient registers new GoogleCloudMessaging client associating with user
// through specfied token.
func RegisterGCMClient(gcmID, token string) (*GCMClient, error) {
	u := new(User)
	if db.Where("token = ?", token).First(u).RecordNotFound() {
		return nil, fmt.Errorf("Token not found")
	}
	g := new(GCMClient)
	if db.Where("gcm_id = ?", gcmID).First(g).RecordNotFound() {
		// If the client doesnt exist, create it
		g = &GCMClient{
			GCMId: gcmID,
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

// TableName is function used with gorm library
func (g GCMClient) TableName() string {
	return "gcm_clients"
}

// Save is shortcut to save object to database
func (g *GCMClient) Save() {
	db.Save(g)
}

// Delete is shortcut to delete object from database
func (g *GCMClient) Delete() {
	db.Delete(g)
}
