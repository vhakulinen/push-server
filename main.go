package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/vhakulinen/push-server/config"
	"github.com/vhakulinen/push-server/db"
)

var configFile = flag.String("config", "push-serv.conf", "Path to config file")

var httpHostPort string

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	email := r.FormValue("email")
	password := r.FormValue("password")
	user, err := db.NewUser(email, password)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("%v", err)))
		return
	}
	t, err := user.HttpToken()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("%v", err)))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%s", t.Token)))
}

func PushHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	title := r.FormValue("title")
	body := r.FormValue("body")
	token := r.FormValue("token")
	stimestamp := r.FormValue("timestamp")
	if stimestamp != "" {
		timestamp, err := strconv.ParseInt(stimestamp, 10, 64)
		if err != nil {
			log.Printf("Failed to parse timestamp int PushHandler() (%v)", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Failed to parse timestamp"))
			return
		} else if timestamp < 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Timestamp can't be less than 0"))
			return
		}
		_, err = db.SavePushData(title, body, token, timestamp)
		if err != nil {
			log.Printf("Something went wrong! (%v)", err)
		}
	} else {
		_, err := db.SavePushDataMinimal(title, body, token)
		if err != nil {
			log.Printf("Something went wrong! (%v)", err)
		}
	}
}

func PoolHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data := ""
	token := r.FormValue("token")
	t, err := db.GetHttpToken(token)
	if err == nil {
		for _, push := range t.GetPushes() {
			tmp, err := push.ToJson()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Something went wrong!"))
				log.Printf("%v", err)
				return
			}
			data += string(tmp)
		}
	}
	w.Write([]byte(data))
}

func RetrieveHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	email := r.FormValue("email")
	password := r.FormValue("password")
	user, err := db.GetUser(email)
	if err != nil || !user.ValidatePassword(password) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(http.StatusText(http.StatusNotFound)))
	} else {
		t, err := user.HttpToken()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
		} else {
			w.Write([]byte(t.Token))
		}
	}
}

func main() {
	flag.Parse()
	config.GetConfig(*configFile)

	logToTty, err := config.Config.Bool("log", "totty")
	logFile, err := config.Config.String("log", "file")
	host, err := config.Config.String("default", "host")
	port, err := config.Config.Int("default", "port")
	certPemFile, err := config.Config.String("ssl", "certpath")
	keyPemFile, err := config.Config.String("ssl", "keypath")

	if err != nil {
		log.Fatal(err)
	}

	httpHostPort = fmt.Sprintf("%s:%d", host, port)

	if !logToTty {
		f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	http.HandleFunc("/register/", RegisterHandler)
	http.HandleFunc("/push/", PushHandler)
	http.HandleFunc("/pool/", PoolHandler)
	http.HandleFunc("/retrieve/", RetrieveHandler)

	if err := http.ListenAndServeTLS(httpHostPort, certPemFile, keyPemFile, nil); err != nil {
		log.Fatal(err)
	}
}
