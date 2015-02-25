package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/vhakulinen/push-server/pushserv"
)

const (
	TokenMinLength = 8
)

var host = flag.String("host", "localhost", "Address to bind")
var httpPort = flag.String("httpport", "8080", "Port to bind for pushing and http pooling")
var logFile = flag.String("logfile", "/var/log/push-server.log", "File to save log data")
var logToTty = flag.Bool("logtty", false, "Output log to tty")
var certPemFile = flag.String("cert", "cert.pem", "Certificate pem file")
var keyPemFile = flag.String("key", "key.pem", "Key pem file")

var httpHostPort string

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	email := r.FormValue("email")
	password := r.FormValue("password")
	user, err := pushserv.NewUser(email, password)
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
		}
		_, err = pushserv.SavePushData(title, body, token, timestamp)
		if err != nil {
			log.Printf("Something went wrong! (%v)", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Something went wrong!"))
		}
		return
	}
	_, err := pushserv.SavePushDataMinimal(title, body, token)
	if err != nil {
		log.Printf("Something went wrong! (%v)", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Something went wrong!"))
	}
}

func PoolHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	token := r.FormValue("token")
	t, err := pushserv.GetHttpToken(token)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(http.StatusText(http.StatusNotFound)))
	} else {
		data := ""
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
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(data))
	}
}

func RetrieveHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	email := r.FormValue("email")
	password := r.FormValue("password")
	user, err := pushserv.GetUser(email)
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
	httpHostPort = fmt.Sprintf("%s:%s", *host, *httpPort)

	if !*logToTty {
		f, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
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
	if err := http.ListenAndServeTLS(httpHostPort, *certPemFile, *keyPemFile, nil); err != nil {
		log.Fatal(err)
	}
}
