package db

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/vhakulinen/push-server/config"
)

func TestMain(m *testing.M) {
	config.GetConfig("../push-serv.conf.def")
	SetupDatabase()
	BackupForTesting()
	code := m.Run()
	RestoreFromTesting()
	os.Exit(code)
}

func TestNewUser(t *testing.T) {
	var testData = []struct {
		Email        string
		Password     string
		ExpectingErr bool
	}{
		{"user@domain.com", "password123", false},
		{"user@domain.com", "dubplicateemail", true},
		{"invalid@password.com", "inva", true},
		{"user@domain.com", "", true},
		{"", "emptyemail", true},
		{"invalidEmail", "password123", true},
		{"a@co", "password123", true},
		{"a@.io", "password123", true},
		{"@domain.com", "password123", true},
		{"username@", "password123", true},
	}

	// Store here and delete afterwards
	users := []*User{}

	for _, data := range testData {
		u, err := NewUser(data.Email, data.Password)

		users = append(users, u)
		if err != nil {
			if !data.ExpectingErr {
				t.Errorf("Got error while not expecting one! (%v)", err)
			}
			continue
		}
		if err == nil && data.ExpectingErr {
			t.Errorf("Was expecting error and didn't get one! Data(%v, %v)", data.Email, data.Password)
		}
		if u.Email != data.Email {
			t.Errorf("Email didn't mach in newly created user! (%v != %v)", data.Email, u.Email)
		}
		if u.Password == data.Password {
			t.Errorf("Password isn't hashed!")
		}
		if u.Active != false {
			t.Error("Active field should default to false!")
		}
	}

	// Cleanup
	for _, u := range users {
		if u != nil {
			db.Unscoped().Delete(u)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	// Use these values to create the user
	const email = "user@domain.com"
	const pass = "validPassword"
	const invalidPass = "invalidPassword"

	u, err := NewUser(email, pass)
	if err != nil {
		t.Fatalf("Couldn't create user in TestValidatePassword! (%v)", err)
	}
	if b := u.ValidatePassword(pass); b != true {
		t.Errorf("Password validation test failed! Expected %v, got %v", true, b)
	}
	if b := u.ValidatePassword(invalidPass); b != false {
		t.Errorf("Password validation test failed! Expected %v, got %v", false, b)
	}

	// Cleanup
	db.Unscoped().Delete(u)
}

func TestSavePushData(t *testing.T) {
	u, err := NewUser("save@pushdata.com", "password")
	if err != nil {
		t.Fatalf("Failed to create user! (%v)", err)
	}
	token := u.Token
	var testData = []struct {
		Title     string
		Body      string
		Token     string
		TimeStamp int64
		Priority  int64

		ExpectingErr bool
	}{
		{"required", "", token, -1, 1, true}, // Timestamp less than 0
		{"required", "", token, 123, 1, false},
		{"required", "", token, 123, 2, false},
		{"required", "", token, 123, 10, false}, // Priority should default to 1
		{"required", "", token, 123, 0, false},  // Priority should default to 1
		{"", "bod", token, 0, 1, true},          // No title
		{"title", "body", "", 0, 1, true},       // To token
		{"title", "body", token, 123, 1, false}, // Everything is good
		{"there is no", "valid token", "invalidtoken", 0, 1, true},
	}

	for _, data := range testData {
		pushData, err := SavePushData(data.Title, data.Body, data.Token,
			data.TimeStamp, data.Priority)
		if err != nil {
			if !data.ExpectingErr {
				t.Errorf("Got error while not expecting one! (%v)", err)
			}
			continue
		} else if data.ExpectingErr {
			t.Errorf("Was expecting error and didn't get any")
		}
		if pushData.Title != data.Title {
			t.Errorf("Titles didn't match! (%v != %v)", data.Title, pushData.Title)
		}
		if pushData.Body != data.Body {
			t.Errorf("Bodies didn't match! (%v != %v)", data.Body, pushData.Body)
		}
		if pushData.UnixTimeStamp != data.TimeStamp {
			t.Errorf("Timestamps didn't match! (%v != %v)", data.TimeStamp, pushData.UnixTimeStamp)
		}
		if pushData.Token != data.Token {
			t.Errorf("Tokens didn't match! (%v != %v)", data.Token, pushData.Token)
		}
		if data.Priority > 3 || data.Priority < 1 {
			if pushData.Priority != 1 {
				t.Errorf("Priority should default to 1 with invaild value! (value was %d)", data.Priority)
			}
		} else if pushData.Priority != data.Priority {
			t.Errorf("Prioties didn't match! (%v != %v)", pushData.Priority, data.Priority)
		}
		if pushData.Sound != true {
			t.Errorf("PushData.Sound should default to true")
		}

		db.Unscoped().Delete(pushData)
	}
}

func TestToJson(t *testing.T) {
	title := "title"
	body := "body"
	var time int64
	time = 1
	type pushData struct {
		Title         string
		Body          string
		UnixTimeStamp int64
	}

	u, err := NewUser("to@jsontest.com", "password")
	if err != nil {
		t.Fatalf("Failed to create user  (%v)\n", err)
	}
	token := u.Token

	pushdata, err := SavePushData(title, body, token, time, 1)
	if err != nil {
		t.Fatalf("Failed to create push data! (%v)", err)
	}
	defer db.Unscoped().Delete(pushdata)

	b, err := pushdata.ToJson()
	v := &pushData{}
	err = json.Unmarshal(b, v)
	if err != nil {
		t.Errorf("Didn't expect error and got one! (%v)", err)
	}
	if v.Body != body {
		t.Errorf("Bodies didn't match! (%v != %v)", body, v.Body)
	}
	if v.Title != title {
		t.Errorf("Titles didn't match! (%v != %v)", title, v.Title)
	}
	if v.UnixTimeStamp != time {
		t.Errorf("Timestamps didn't match! (%v != %v)", time, v.UnixTimeStamp)
	}

	db.Unscoped().Delete(pushdata)
}
