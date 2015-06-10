package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/vhakulinen/push-server/config"
	"github.com/vhakulinen/push-server/db"
	"github.com/vhakulinen/push-server/email"
	"github.com/vhakulinen/push-server/tcp"
	"github.com/vhakulinen/push-server/utils"
)

const (
	tokenRegexString = "[0-9a-zA-Z]{8}-[0-9a-zA-Z]{4}-[0-9a-zA-Z]{4}-" +
		"[0-9a-zA-Z]{4}-[0-9a-zA-Z]{12}"
	emailpassRequiredRegexString = "Email and password required"
	userExistsRegexString        = "User exists"
)

func TestMain(m *testing.M) {
	config.GetConfig("push-serv.conf.def")
	db.SetupDatabase()
	db.BackupForTesting()

	// General mock for these functions
	email.SendRegistrationEmail = func(u *db.User) error { return nil }
	utils.SendGcmPing = func(regIds []string) { return }

	code := m.Run()
	db.RestoreFromTesting()
	os.Exit(code)
}

func TestRetrieveHandler(t *testing.T) {
	var email = "retrieve@user.com"
	var pass = "password"
	var token string

	ts := httptest.NewServer(http.HandlerFunc(RetrieveHandler))
	defer ts.Close()

	// Add new user
	user, err := db.NewUser(email, pass)
	if err != nil {
		t.Fatalf("Failed to add user! (%v)", err)
	}
	user.Activate()

	token = user.Token

	var testData = []struct {
		email          string
		password       string
		expectedCode   int
		expectedString string
	}{
		{email, pass, 200, token},
		{email, "invalidpass", 404, http.StatusText(http.StatusNotFound)},
		{"invalid", "pass", 404, http.StatusText(http.StatusNotFound)},
	}

	for i, data := range testData {
		form := url.Values{}
		form.Add("email", data.email)
		form.Add("password", data.password)

		res, err := http.PostForm(ts.URL, form)
		if err != nil {
			t.Fatal(err)
		}

		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != data.expectedCode {
			t.Errorf("Got %d, want %d (run %d)", res.StatusCode, data.expectedCode, i)
		}
		if string(body) != data.expectedString {
			t.Errorf("Got \"%v\", want \"%s\" (run %d)", string(body), data.expectedString, i)
		}
	}
}

func TestActivateUserHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(ActivateUserHandler))
	defer ts.Close()

	// Register the user
	email := "activateuser@domain.com"

	user, err := db.NewUser(email, "password123")
	if err != nil {
		t.Fatal(err)
	}

	var testData = []struct {
		Email        string
		Key          string
		ExpectedCode int
	}{
		{user.Email, "foo", 400},
		{"fail@domain.com", user.ActivateToken, 400},
		{user.Email, user.ActivateToken, 200},
	}

	for _, data := range testData {
		res, err := http.Get(fmt.Sprintf("%s?email=%s&key=%s", ts.URL, data.Email, data.Key))
		if err != nil {
			t.Fatal(err)
		}
		if data.ExpectedCode != res.StatusCode {
			t.Errorf("Expected %v, got %v instead", data.ExpectedCode, res.StatusCode)
		}
	}
	user, err = db.GetUser(email)
	if err != nil {
		t.Fatalf("Failed to get user with GetUser(%v)", email)
	}
	if user.Active == false {
		t.Error("User.Active == false after activation!")
	}
	res, _ := http.Get(fmt.Sprintf("%s/", ts.URL))
	if res.StatusCode != 400 {
		t.Error("URI with no email or key should have returned 400")
	}
	res, _ = http.Get(fmt.Sprintf("%s/asd/", ts.URL))
	if res.StatusCode != 400 {
		t.Error("URI with ivalid data should have returned 400")
	}
}

func TestRegisterHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(RegisterHandler))
	defer ts.Close()

	emailDone := false
	// Mock for this test
	// NOTE: We'll are only waiting for one valid email address
	email.SendRegistrationEmail = func(u *db.User) error {
		if u.Active != false {
			t.Error("User.Active should be false!")
		}
		expecting := "valid@email.com"
		if u.Email != expecting {
			t.Errorf("Expected %v, got %v instead", expecting, u.Email)
		} else {
			emailDone = true
		}
		return nil
	}

	var testData = []struct {
		email          string
		password       string
		expectedCode   int
		expectedString string
	}{
		{"", "", 400, emailpassRequiredRegexString},
		{"email@isnot_enough.com", "", 400, emailpassRequiredRegexString},
		{"", "passwordisnotenough", 400, emailpassRequiredRegexString},
		{"valid@email.com", "validpassword", 200, "Activation link was sent by email"},
		{"valid@email.com", "validpassword", 400, userExistsRegexString},
	}

	for i, data := range testData {
		form := url.Values{}
		form.Add("email", data.email)
		form.Add("password", data.password)

		res, err := http.PostForm(ts.URL, form)
		if err != nil {
			t.Fatal(err)
		}

		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != data.expectedCode {
			t.Errorf("Got %d, want %d (run %d)", res.StatusCode, data.expectedCode, i)
		}
		ok, err := regexp.Match(data.expectedString, body)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("Got \"%s\", want string matching regex \"%s\" (run %d)", body, data.expectedString, i)
		}
	}

	// Check that SendRegistrationEmail was ran
	if !emailDone {
		t.Error("email.SendRegistrationEmail was not called!")
	}

	// Test the skipEmailVerification = true
	//config.Config.AddOption("registration", "skipEmailVerification", "true")
	skipEmailVerification = true // Simulate the above
	semail := "skip@activation.com"
	form := url.Values{}
	form.Add("email", semail)
	form.Add("password", "password123")

	res, err := http.PostForm(ts.URL, form)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	ok, err := regexp.Match(tokenRegexString, body)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("Got \"%s\", want string matching regex \"%s\"", body, tokenRegexString)
	}

	// Check the user.Active state
	user, err := db.GetUser(semail)
	if err != nil {
		t.Fatal(err)
	}
	if user.Active != true {
		t.Errorf("User.Active should be true since skipEmailVerification option was set to true in configuration!")
	}
	skipEmailVerification = false // Reset
}

func TestPushHandler(t *testing.T) {
	/*
		PushHander doesnt care if the token is valid or not, so we dont check
		the return messages because those will only contain error messages
		to user, if even those.

		Also, was the priority whatever, it shouldn't cause any problem
	*/

	ts := httptest.NewServer(http.HandlerFunc(PushHandler))
	defer ts.Close()

	// Backup to restore
	oClientFromPool := tcp.ClientFromPool
	defer func() {
		tcp.ClientFromPool = oClientFromPool
	}()

	tcpcount := 0
	tcp.ClientFromPool = func(token string) (chan<- string, bool) {
		tcpcount++
		return nil, false
	}

	u, err := db.NewUser("push@test1.com", "password")
	if err != nil {
		t.Fatalf("Failed to craete new user (%v)", err)
	}

	var testData = []struct {
		title        string
		body         string
		token        string
		timestamp    string
		priority     string
		expectedCode int
	}{
		// Server doesnt notify user if the token is invalid
		// so this takes care of invalid and valid token situations

		{"title", "body", u.Token, "", "on", 200},
		{"title", "body", u.Token, "", "10", 200},
		{"title", "body", u.Token, "", "-1", 200},
		{"title", "body", u.Token, "", "1", 200},
		{"title", "body", u.Token, "", "2", 200},
		{"title", "body", u.Token, "", "3", 200},

		// All cases below are expected to fail
		{"title", "body", "invalidtoken", "", "10", 200},

		{"title", "body", "token", "invalidtimestapm", "", 200},
		{"title", "body", "token", "-11", "", 200},
		{"", "noTokenNorTitle", "", "", "2", 200},
		{"", "noTokenNorTitle", "", "", "3", 200},
		{"", "noTokenNorTitleWithTimeStamp", "", "100", "nn", 200},
	}

	for i, data := range testData {
		form := url.Values{}
		form.Add("title", data.title)
		form.Add("body", data.body)
		form.Add("token", data.token)
		form.Add("timestamp", data.timestamp)
		form.Add("priority", data.priority)

		res, err := http.PostForm(ts.URL, form)
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != data.expectedCode {
			t.Errorf("Got %d, want %d (run %d)", res.StatusCode, data.expectedCode, i)
		}
	}

	if tcpcount != 5 {
		t.Errorf("tcp.ClientFromPool call count was unexpected (expected %v, got %v)", 5, tcpcount)
	}

	count := 0
	id1 := false
	id2 := false
	utils.SendGcmPing = func(regIds []string) {
		for _, id := range regIds {
			switch id {
			case "id1":
				id1 = true
				break
			case "id2":
				id2 = true
				break
			default:
				t.Errorf("Got unexpected ID in utils.SendGcmPing (%v)", id)
				break
			}
		}
		count++
	}

	u, err = db.NewUser("gen@user.com", "password")
	if err != nil {
		t.Fatal(err)
	}

	db.RegisterGCMClient("id1", u.Token)
	db.RegisterGCMClient("id2", u.Token)

	form := url.Values{}
	form.Add("title", "title")
	form.Add("body", "body")
	form.Add("token", u.Token)
	form.Add("timestamp", "0")

	_, err = http.PostForm(ts.URL, form)
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Errorf("utils.SendGcmPing was not called!")
	}
	if !id1 || !id2 {
		t.Errorf("utils.SendGcmPing was called with invalid IDs!")
	}
}

func TestPoolHandler(t *testing.T) {
	var pushToken string
	var pushTitle = "title"
	var pushBody = "body"
	var pushUrl = "www.ddg.gg"
	var pushTime = time.Now().Unix()

	ts := httptest.NewServer(http.HandlerFunc(PoolHandler))
	defer ts.Close()

	// Add user
	user, err := db.NewUser("pooluser@domain.com", "password")
	if err != nil {
		t.Fatalf("Failed to create user! (%v)", err)
	}
	pushToken = user.Token

	// Add push data
	_, err = db.SavePushData(pushTitle, pushBody, pushToken, pushUrl, pushTime, 1)
	if err != nil {
		t.Fatal(err)
	}

	// The actual testing
	var testData = []struct {
		token          string
		expectingValid bool
		expectedCode   int
	}{
		{pushToken, true, 200},
		{"invalidtoken", false, 200},
		{"invalid", false, 200},
	}
	type validDataStrcut struct {
		UnixTimeStamp int64
		Title         string
		Body          string
		Url           string
	}

	for i, data := range testData {
		form := url.Values{}
		form.Add("token", data.token)

		res, err := http.PostForm(ts.URL, form)
		if err != nil {
			t.Fatal(err)
		}
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != data.expectedCode {
			t.Errorf("Go %v status code, want %d (run %d)", res.StatusCode, data.expectedCode, i)
		}

		if data.expectingValid {
			v := &validDataStrcut{}
			err = json.Unmarshal(body, v)
			if err != nil {
				t.Fatal(err)
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
			if v.Url != pushUrl {
				t.Errorf("Got \"%v\" in time, want \"%v\"", v.Url, pushUrl)
			}
		}
	}
}

func TestGCMRegisterHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(GCMRegisterHandler))
	defer ts.Close()

	u1, err := db.NewUser("gcm1@gcm.com", "password")
	u2, err := db.NewUser("gcm2@gcm.com", "password")
	if err != nil {
		t.Fatalf("Failed to create users (%v)", err)
	}

	token := u1.Token
	token2 := u2.Token
	var testData = []struct {
		token        string
		gcmid        string
		expectedCode int
	}{
		{"", "", 400},
		{token, "gcmid", 200},
		{token, "gcmid", 200},  // Same token, should just pass
		{token2, "gcmid", 200}, // Update the token
		{token, "gcmid2", 200},
		{"footoken", "foobar", 500}, // invalid token
	}

	for _, data := range testData {
		form := url.Values{}
		form.Add("token", data.token)
		form.Add("gcmid", data.gcmid)

		res, err := http.PostForm(ts.URL, form)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()

		if res.StatusCode != data.expectedCode {
			t.Errorf("Expected %v but got %v instead!", data.expectedCode, res.StatusCode)
		}
	}
}

func TestGMCUnregisterHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(GCMUnregisterHandler))
	defer ts.Close()

	gcmId := "GCMID"

	u, err := db.NewUser("unregistergcm@gcm.com", "password")
	if err != nil {
		t.Fatalf("Failed to create user (%v)", err)
	}
	_, err = db.RegisterGCMClient(gcmId, u.Token)
	if err != nil {
		t.Fatalf("Failed to create gcm client (%v)", err)
	}

	var testData = []struct {
		gcmid        string
		expectedCode int
		entryDeleted bool // flag to check if the entry should be removed
	}{
		{"invalidToken", 200, false},
		{gcmId, 200, true},
	}

	for _, d := range testData {
		// Valid - this sould remove the gcm client
		form := url.Values{}
		form.Add("gcmid", d.gcmid)
		res, err := http.PostForm(ts.URL, form)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != d.expectedCode {
			t.Errorf("Expected %s but got %s instead!", 200, res.StatusCode)
		}

		_, err = db.GetGCMClient(gcmId)
		if d.entryDeleted {
			if err == nil {
				t.Errorf("GCMClient should be deleted but is not")
			}
		} else {
			if err != nil {
				t.Errorf("GCMClient shouldn't be deleted but is")
			}
		}
	}
}
