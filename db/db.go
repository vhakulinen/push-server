package db

import (
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

var db gorm.DB

func GetAllPushDatas() []PushData {
	pushdatas := []PushData{}
	db.Find(&pushdatas)
	return pushdatas
}

func GetHttpToken(token string) (t *HttpToken, err error) {
	t = new(HttpToken)
	if db.Where("token = ?", token).First(t).RecordNotFound() {
		return nil, fmt.Errorf("Token not found")
	}
	return t, nil
}

func GetAllTokens() []HttpToken {
	tokens := []HttpToken{}
	db.Find(&tokens)
	return tokens
}

func GetUser(email string) (*User, error) {
	u := new(User)
	if db.Where("email = ?", email).First(u).RecordNotFound() {
		return nil, fmt.Errorf("User not found")
	}
	return u, nil
}

// This is just for testing purposes
func SetupDatabase() gorm.DB {
	var err error
	db, err = gorm.Open("sqlite3", "db.sqlite3")
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&PushData{})
	db.AutoMigrate(&HttpToken{})
	db.AutoMigrate(&User{})
	return db
}

func init() {
	SetupDatabase()
}
