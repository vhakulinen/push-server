package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/vhakulinen/push-server/pushserv"
)

const (
	tokenRegexString = "[0-9a-zA-Z]{8}-[0-9a-zA-Z]{4}-[0-9a-zA-Z]{4}-" +
		"[0-9a-zA-Z]{4}-[0-9a-zA-Z]{12}:[0-9a-zA-Z\\-]{6}"
	emailpassRequiredRegexString = "Email and password required"
	userExistsRegexString        = "User exists"
)

func TestMain(m *testing.M) {
	os.Rename("db.sqlite3", "db.sqlite3.backup")
	pushserv.SetupDatabase()
	code := m.Run()
	os.Rename("db.sqlite3.backup", "db.sqlite3")
	os.Exit(code)
}

func TestRegisterHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(RegisterHandler))
	defer ts.Close()

	var testData = []struct {
		email          string
		password       string
		expectedCode   int
		expectedString string
	}{
		{"", "", 400, emailpassRequiredRegexString},
		{"emailisnotenough", "", 400, emailpassRequiredRegexString},
		{"", "passwordisnotenough", 400, emailpassRequiredRegexString},
		{"validemail", "validpassword", 200, tokenRegexString},
		{"validemail", "validpassword", 400, userExistsRegexString},
	}

	for i, data := range testData {
		form := url.Values{}
		form.Add("email", data.email)
		form.Add("password", data.password)

		// res, err := http.Get(ts.URL)
		res, err := http.PostForm(ts.URL, form)
		if err != nil {
			log.Fatal(err)
		}

		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}

		if res.StatusCode != data.expectedCode {
			t.Errorf("Got %d, want %d (run %d)", res.StatusCode, data.expectedCode, i)
		}
		ok, err := regexp.Match(data.expectedString, body)
		if err != nil {
			log.Fatal(err)
		}
		if !ok {
			t.Errorf("Got \"%s\", want string matching regex \"%s\" (run %d)", body, data.expectedString, i)
		}
	}
}
