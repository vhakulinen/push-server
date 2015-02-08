package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/vhakulinen/push-server/pushserv"
)

const (
	tokenRegexString = "[0-9a-zA-Z]{8}-[0-9a-zA-Z]{4}-[0-9a-zA-Z]{4}-" +
		"[0-9a-zA-Z]{4}-[0-9a-zA-Z]{12}"
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

func TestRetrieveHandler(t *testing.T) {
	var user = "retrieveuser"
	var pass = "password"
	var token string

	ts := httptest.NewServer(http.HandlerFunc(RetrieveHandler))
	defer ts.Close()

	// Register user
	tsregsiter := httptest.NewServer(http.HandlerFunc(RegisterHandler))
	defer ts.Close()

	form := url.Values{}
	form.Add("email", user)
	form.Add("password", pass)

	res, err := http.PostForm(tsregsiter.URL, form)
	if err != nil {
		log.Fatal(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	token = string(body)

	var testData = []struct {
		email          string
		password       string
		expectedCode   int
		expectedString string
	}{
		{user, pass, 200, token},
		{user, "invalidpass", 404, http.StatusText(http.StatusNotFound)},
		{"invalid", "pass", 404, http.StatusText(http.StatusNotFound)},
	}

	for i, data := range testData {
		form = url.Values{}
		form.Add("email", data.email)
		form.Add("password", data.password)

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
		if string(body) != data.expectedString {
			t.Errorf("Got \"%v\", want \"%s\" (run %d)", string(body), data.expectedString, i)
		}
	}
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

func TestPushHandler(t *testing.T) {
	/*
		PushHander doesnt care if the token is valid or not, so we dont check
		the return messages because those will only contain error messages
		to user, if even those.
	*/

	ts := httptest.NewServer(http.HandlerFunc(PushHandler))
	defer ts.Close()

	var testData = []struct {
		title        string
		body         string
		token        string
		timestamp    string
		expectedCode int
	}{
		// Server doesnt notify user if the token is invalid
		// so this takes care of invalid and valid token situations
		{"title", "body", "invalidtoken", "", 200},

		{"title", "body", "token", "invalidtimestapm", 400},
		{"", "noTokenNorTitle", "", "", 500},
		{"", "noTokenNorTitleWithTimeStamp", "", "100", 500},
	}

	for i, data := range testData {
		form := url.Values{}
		form.Add("title", data.title)
		form.Add("body", data.body)
		form.Add("token", data.token)
		form.Add("timestamp", data.timestamp)

		res, err := http.PostForm(ts.URL, form)
		if err != nil {
			log.Fatal(err)
		}

		if res.StatusCode != data.expectedCode {
			t.Errorf("Got %d, want %d (run %d)", res.StatusCode, data.expectedCode, i)
		}
	}
}

func TestPoolHandler(t *testing.T) {
	var pushToken string
	var pushTitle = "title"
	var pushBody = "body"
	var pushTime = time.Now().Unix()

	ts := httptest.NewServer(http.HandlerFunc(PoolHandler))
	defer ts.Close()

	// We need to push atleast one pushdata to be able to test the pooling
	tspush := httptest.NewServer(http.HandlerFunc(PushHandler))
	defer ts.Close()
	// And we need to regsiter user for that
	tsregsiter := httptest.NewServer(http.HandlerFunc(RegisterHandler))
	defer ts.Close()

	// Register the user
	form := url.Values{}
	form.Add("email", "user")
	form.Add("password", "password")
	res, err := http.PostForm(tsregsiter.URL, form)
	if err != nil {
		log.Fatal(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	pushToken = string(body)

	// Push some data
	form = url.Values{}
	form.Add("token", pushToken)
	form.Add("title", pushTitle)
	form.Add("body", pushBody)
	form.Add("timestamp", strconv.FormatInt(pushTime, 10))
	_, err = http.PostForm(tspush.URL, form)
	if err != nil {
		log.Fatal(err)
	}

	// The actual testing
	var testData = []struct {
		token          string
		expectingValid bool
		expectedCode   int
	}{
		{pushToken, true, 200},
		{"invalidtoken", false, 404},
		{"invalid", false, 404},
	}
	type validDataStrcut struct {
		UnixTimeStamp int64
		Title         string
		Body          string
	}

	for i, data := range testData {
		form = url.Values{}
		form.Add("token", data.token)

		res, err = http.PostForm(ts.URL, form)
		if err != nil {
			log.Fatal(err)
		}
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}

		if res.StatusCode != data.expectedCode {
			t.Errorf("Go %v status code, want %d (run %d)", res.StatusCode, data.expectedCode, i)
		}

		if data.expectingValid {
			v := &validDataStrcut{}
			err = json.Unmarshal(body, v)
			if err != nil {
				log.Fatal(err)
			}
			if v.Body != pushBody {
				t.Errorf("Got \"%v\" in body, want \"%s\"", v.Body, pushBody)
			}
			if v.Title != pushTitle {
				t.Errorf("Got \"%v\" in title, want \"%s\"", v.Title, pushTitle)
			}
			if v.UnixTimeStamp != pushTime {
				t.Errorf("Got \"%v\" in time, want \"%d\"", v.UnixTimeStamp, pushTime)
			}
		}
	}
}
