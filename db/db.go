package db

import (
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	// Load postgres
	_ "github.com/lib/pq"
	// Load sqlite
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

// GetAllPushDatas returns /all/ PushData objects in database.
func GetAllPushDatas() []PushData {
	pushdatas := []PushData{}
	db.Find(&pushdatas)
	return pushdatas
}

// GetGCMClient returns GCMClient object if found with specified identifier.
func GetGCMClient(id string) (*GCMClient, error) {
	g := new(GCMClient)
	if db.Where("gcm_id = ?", id).First(g).RecordNotFound() {
		return nil, fmt.Errorf("Client not found")
	}
	return g, nil
}

// GetUser returns User object if found with specified email.
func GetUser(email string) (*User, error) {
	u := new(User)
	if db.Where("email = ?", email).First(u).RecordNotFound() {
		return nil, fmt.Errorf("User not found")
	}
	return u, nil
}

// GetUserByToken returns User object if found with specified token.
func GetUserByToken(token string) (*User, error) {
	u := new(User)
	if db.Where("token = ?", token).First(u).RecordNotFound() {
		return nil, fmt.Errorf("Token doesn't exists")
	}
	return u, nil
}

// TokenExists returns boolean indicating if specified token exists.
func TokenExists(token string) bool {
	u := new(User)
	if db.Where("token = ?", token).First(u).RecordNotFound() {
		return false
	}
	return true
}

// GetPushesForToken returns PushData objects linked to specified token.
func GetPushesForToken(token string) []PushData {
	out := []PushData{}
	u, err := GetUserByToken(token)
	if err == nil {
		db.Where("token = ?", u.Token).Find(&out)
	}
	return out
}

// SetupDatabase is just for testing purposes
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

// BackupForTesting creates backup of current database before running tests.
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

// RestoreFromTesting restores the database which was backedup before running tests.
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
