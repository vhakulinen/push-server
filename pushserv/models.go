package pushserv

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"crypto/rand"
	"crypto/sha256"

	"code.google.com/p/go-uuid/uuid"
)

const (
	DefaultGenKeySize  = 6
	MinPasswordLength  = 6
	PasswordSaltLength = 16
)

type User struct {
	Id         int64
	CreatedAt  time.Time
	ModifiedAt time.Time
	DeletedAt  time.Time

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
	bsalt := make([]byte, PasswordSaltLength)
	if len(u.Password) < MinPasswordLength {
		return errors.New("Password is too short")
	}
	_, err := rand.Read(bsalt)
	if err != nil {
		log.Printf("Error in User.BeforeCreate() (%v)", err)
		return errors.New("Something went wrong!")
	}
	salt := string(bsalt)
	b := sha256.Sum256([]byte(u.Password + salt))
	u.Password = salt + string(b[:])
	return nil
}

func (u *User) ValidatePassowrd(password string) bool {
	hash := sha256.Sum256([]byte(password + u.Password[:PasswordSaltLength]))
	if len(u.Password) > PasswordSaltLength+MinPasswordLength {
		if string(hash[:]) == u.Password[PasswordSaltLength:] {
			return true
		}
	} else {
		log.Println("Invalid password in database (password length is too short)")
	}
	return false
}

func (u *User) GetHttpToken() (*HttpToken, error) {
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

	Token string `sql:"not null;unique"`
	Key   string `sql:"not null"`
}

func GenerateAndSaveToken() (*HttpToken, error) {
	t := new(HttpToken)
	var id string
	var keyraw []byte
	count := 0
	for {
		id = uuid.NewUUID().String()
		keyraw = make([]byte, DefaultGenKeySize)
		_, err := rand.Read(keyraw)
		if err != nil {
			log.Fatal(err)
		}
		key := base64.URLEncoding.EncodeToString(keyraw)
		if t, err = RegisterHttpToken(id, key); err == nil {
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
func RegisterHttpToken(token, key string) (t *HttpToken, err error) {
	t = new(HttpToken)
	if db.Where("token = ?", token).First(t).RecordNotFound() {
		t = &HttpToken{
			Token:      token,
			Key:        key,
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

type PushData struct {
	Id        int64
	CreatedAt time.Time
	DeletedAt time.Time

	UnixTimeStamp int64
	Title         string `sql:"not null"`
	Body          string
	Token         string `sql:"not null"`
}

func SavePushData(title, body, token string, timestamp int64) (p *PushData, err error) {
	if timestamp < 0 {
		return nil, fmt.Errorf("Timestamp can't be less than 0")
	}
	if title == "" || token == "" {
		return nil, fmt.Errorf("token and title required")
	}
	p = &PushData{
		Title:         title,
		Body:          body,
		Token:         token,
		UnixTimeStamp: timestamp,
	}
	if err = db.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func SavePushDataMinimal(title, body, token string) (p *PushData, err error) {
	if title == "" || token == "" {
		return nil, fmt.Errorf("token and title required")
	}
	p = &PushData{
		Title: title,
		Body:  body,
		Token: token,
	}
	if err = db.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
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
