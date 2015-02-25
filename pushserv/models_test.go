package pushserv

import (
	"encoding/json"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Rename("db.sqlite3", "db.sqlite3.backup")
	SetupDatabase()
	code := m.Run()
	os.Rename("db.sqlite3.backup", "db.sqlite3")
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
		{"emptypass", "", true},
		{"", "emptyemail", true},
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

func TestRegisterHttpToken(t *testing.T) {
	var tokenStr = "oekfokefokef"
	token, err := RegisterHttpToken(tokenStr)
	if err != nil {
		t.Errorf("Couldn't register token! (%v)", err)
	}
	_, err = RegisterHttpToken(tokenStr)
	if err == nil {
		t.Errorf("Expected error while duplicating token, didn't get one!")
	}

	db.Unscoped().Delete(token)
}

func TestSavePushData(t *testing.T) {
	token, err := GenerateAndSaveToken()
	if err != nil {
		t.Fatalf("Failed to generate token! (%v)", err)
	}
	var testData = []struct {
		Title     string
		Body      string
		Token     string
		TimeStamp int64

		ExpectingErr bool
	}{
		{"required", "", token.Token, -1, true}, // Timestamp less than 0
		{"required", "", token.Token, 123, false},
		{"", "bod", token.Token, 0, true},          // No title
		{"title", "body", "", 0, true},             // To token
		{"title", "body", token.Token, 123, false}, // Everything is good
	}

	for _, data := range testData {
		pushData, err := SavePushData(data.Title, data.Body, data.Token, data.TimeStamp)
		if err != nil {
			if !data.ExpectingErr {
				t.Errorf("Got error while not expecting one! (%v)", err)
			}
			continue
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

	token, err := SavePushData(title, body, "token", time)
	if err != nil {
		t.Fatalf("Failed to create push data! (%v)", err)
	}
	defer db.Unscoped().Delete(token)

	b, err := token.ToJson()
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

	db.Unscoped().Delete(token)
}
