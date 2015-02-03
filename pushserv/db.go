package pushserv

import (
	"fmt"
	"log"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

var db gorm.DB

func GetAllPushDatas() []*PushData {
	pushdatas := []*PushData{}
	db.Find(&pushdatas)
	return pushdatas
}

// Queries db for tokens and returns one of token and key matches
func GetHttpToken(token, key string) (t *HttpToken, err error) {
	t = new(HttpToken)
	if db.Where("token = ?", token).First(t).RecordNotFound() {
		return nil, fmt.Errorf("Invalid key or token not found")
	}
	if key != t.Key {
		return nil, fmt.Errorf("Invalid key or token not found")
	}
	db.Save(t)
	t.AccessedAt = time.Now()
	if err := db.Save(t).Error; err != nil {
		log.Printf("Cannot save HttpToken (%v)", err)
	}
	return t, nil
}

func GetAllTokens() []*HttpToken {
	tokens := []*HttpToken{}
	db.Find(&tokens)
	return tokens
}

func init() {
	var err error
	db, err = gorm.Open("sqlite3", "db.sqlite3")
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&PushData{})
	db.AutoMigrate(&HttpToken{})
}
