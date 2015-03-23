package db

import (
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/vhakulinen/push-server/config"
)

// For testing
const (
	userTableTemp   = "user_temp"
	pushTableTemp   = "push_temp"
	clientTableTemp = "client_temp"
)

// For testing
var (
	restoreUser   = false
	restorePush   = false
	restoreClient = false
)

var db gorm.DB

func GetAllPushDatas() []PushData {
	pushdatas := []PushData{}
	db.Find(&pushdatas)
	return pushdatas
}

func GetUser(email string) (*User, error) {
	u := new(User)
	if db.Where("email = ?", email).First(u).RecordNotFound() {
		return nil, fmt.Errorf("User not found")
	}
	return u, nil
}

func GetUserByToken(token string) (*User, error) {
	u := new(User)
	if db.Where("token = ?", token).First(u).RecordNotFound() {
		return nil, fmt.Errorf("Token doesn't exists")
	}
	return u, nil
}

func TokenExists(token string) bool {
	u := new(User)
	if db.Where("token = ?", token).First(u).RecordNotFound() {
		return false
	}
	return true
}

func GetPushesForToken(token string) []PushData {
	out := []PushData{}
	u, err := GetUserByToken(token)
	if err == nil {
		db.Where("token = ?", u.Token).Find(&out)
	}
	return out
}

// This is just for testing purposes
func SetupDatabase() gorm.DB {
	var err error
	dbtype, err := config.Config.String("database", "type")
	name, err := config.Config.String("database", "name")
	username, err := config.Config.String("database", "username")
	password, err := config.Config.String("database", "password")
	if err != nil {
		log.Fatal(err)
	}
	switch dbtype {
	case "sqlite3":
		db, err = gorm.Open("sqlite3", name)
		db.Exec("PRAGMA foreign_keys = ON")
		break
	case "postgres":
		db, err = gorm.Open("postgres",
			fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", username, password, name))
		break
	default:
		log.Fatal("Invalid database type!")
		break
	}
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&User{})
	db.AutoMigrate(&PushData{})
	db.AutoMigrate(&GCMClient{})
	return db
}

func BackupForTesting() {
	if ok := db.HasTable(&User{}); ok {
		restoreUser = true
		renameTable("users", userTableTemp)
		db.CreateTable(&User{})
	}
	if ok := db.HasTable(&PushData{}); ok {
		restorePush = true
		renameTable("push_datas", pushTableTemp)
		db.CreateTable(&PushData{})
	}
	if ok := db.HasTable(&GCMClient{}); ok {
		restoreClient = true
		renameTable("gcm_clients", clientTableTemp)
		db.CreateTable(&GCMClient{})
	}
}

func RestoreFromTesting() {
	if restoreUser {
		dropTable("users")
		renameTable(userTableTemp, "users")
	}
	if restorePush {
		dropTable("push_datas")
		renameTable(pushTableTemp, "push_datas")
	}
	if restoreClient {
		dropTable("gcm_clients")
		renameTable(clientTableTemp, "gcm_clients")
	}
}

func renameTable(from, to string) {
	db.Exec(fmt.Sprintf("ALTER TABLE %v RENAME TO %v", from, to))
}

func dropTable(name string) {
	db.Exec(fmt.Sprintf("DROP TABLE %s", name))
}
