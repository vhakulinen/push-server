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

	Email       string `sql:"not null;unique"`
	Password    string
	HttpTokenId int64 `sql:"not null;unique"`
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
		token, err := GenerateAndSaveToken()
		if err != nil {
			return nil, err
		}
		u = &User{
			Email:       email,
			Password:    password,
			HttpTokenId: token.Id,
		}
		if err := db.Save(u).Error; err != nil {
			token.Delete()
			log.Printf("Error in NewUser() (%v)", err)
			return nil, fmt.Errorf("Something went wrong!")
		}
		token.UserId = u.Id
		token.Save()
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

func (u *User) Activate() {
	u.Active = true
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

func (u *User) HttpToken() (*HttpToken, error) {
	t := new(HttpToken)
	if db.Where("id = ?", u.HttpTokenId).First(t).RecordNotFound() {
		return nil, fmt.Errorf("Token not found")
	}
	return t, nil
}

type HttpToken struct {
	Id         int64
	CreatedAt  time.Time
	AccessedAt time.Time
	DeletedAt  time.Time

	UserId int64 `sql:"not null"`

	Token      string `sql:"not null;unique"`
	GCMClients []GCMClient
}

func GenerateAndSaveToken() (*HttpToken, error) {
	var id string
	count := 0
	for {
		id = uuid.NewUUID().String()
		if t, err := RegisterHttpToken(id); err == nil {
			return t, nil
		} else {
			if count >= 3 {
				log.Printf("Error in GenerateAndSaveToken() (%v)", err)
				return nil, fmt.Errorf("Couldn't generate new token!")
			}
			count++
		}
	}
}

// Register token for http pooling
func RegisterHttpToken(token string) (t *HttpToken, err error) {
	t = new(HttpToken)
	if db.Where("token = ?", token).First(t).RecordNotFound() {
		t = &HttpToken{
			Token:      token,
			AccessedAt: time.Now(),
		}
		if err = db.Save(t).Error; err != nil {
			return nil, err
		}
		return t, nil
	}
	return nil, fmt.Errorf("HttpToken already registered")
}

func (t *HttpToken) Save() {
	db.Save(t)
}

func (t *HttpToken) Delete() {
	db.Delete(t)
}

// NOTE: This updates AccessedAt time!
func (t *HttpToken) GetPushes() []PushData {
	pushes := []PushData{}
	db.Where("token = ?", t.Token).Find(&pushes)

	// Soft delete fetched push datas
	for _, p := range pushes {
		db.Delete(p)
	}

	// Update the AccessedAt time
	t.AccessedAt = time.Now()
	t.Save()
	return pushes
}

func (t *HttpToken) AfterFind() {
	gcmClients := []GCMClient{}
	db.Where("token = ?", t.Token).Find(&gcmClients)
	t.GCMClients = gcmClients
}

type PushData struct {
	Id        int64     `json:"-"`
	CreatedAt time.Time `json:"-"`
	DeletedAt time.Time `json:"-"`

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
	_, err = GetHttpToken(token)
	if err != nil {
		return nil, err
	}

	p = &PushData{
		Title:         title,
		Body:          body,
		Token:         token,
		UnixTimeStamp: timestamp,
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
	t := new(HttpToken)
	if db.Where("token = ?", token).First(t).RecordNotFound() {
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
		t.GCMClients = append(t.GCMClients, *g)
		t.Save()
		return g, nil
	} else if g.Token == t.Token {
		// Same token as before, so let it be
		return nil, nil
	} else {
		// If the client has already registered, update the token
		// But before that, delete the GCMClient from the old token's client list
		oldt := new(HttpToken)
		if !db.Where("token = ?", g.Token).First(oldt).RecordNotFound() {
			var pos = -1
			for i, client := range oldt.GCMClients {
				if client.GCMId == g.GCMId {
					pos = i
					break
				}
			}
			if pos != -1 {
				oldt.GCMClients = append(oldt.GCMClients[:pos], oldt.GCMClients[pos+1:]...)
				oldt.Save()
			}
		}
		g.Token = token
		g.Save()
		t.GCMClients = append(t.GCMClients, *g)
		t.Save()
		return g, nil
	}
}

func (g GCMClient) TableName() string {
	return "gcm_clients"
}

func (g *GCMClient) Save() {
	db.Save(g)
}
