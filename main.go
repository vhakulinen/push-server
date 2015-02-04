package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/vhakulinen/push-server/pushserv"
)

const (
	TokenMinLength = 8
	KeyMinLength   = 5
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
	t, err := user.GetHttpToken()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("%v", err)))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%s:%s", t.Token, t.Key)))
}

func PushHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, err := pushserv.SavePushData(r.FormValue("title"), r.FormValue("body"), r.FormValue("token"))
	if err != nil {
		log.Printf("Something went wrong! (%v)", err)
	}
}

func PoolHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	token := r.FormValue("token")
	key := r.FormValue("key")
	t, err := pushserv.GetHttpToken(token, key)
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
	if err := http.ListenAndServeTLS(httpHostPort, *certPemFile, *keyPemFile, nil); err != nil {
		log.Fatal(err)
	}
}
