package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/vhakulinen/push-server/config"
	"github.com/vhakulinen/push-server/db"
	"github.com/vhakulinen/push-server/email"
	"github.com/vhakulinen/push-server/tcp"
	"github.com/vhakulinen/push-server/utils"
)

var configFile = flag.String("config", "push-serv.conf", "Path to config file")

var httpHostPort string
var skipEmailVerification bool

func activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var writeBadRequest = func() {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(http.StatusText(http.StatusBadRequest)))
	}
	defer r.Body.Close()
	err := r.ParseForm()
	if err != nil {
		writeBadRequest()
		return
	}
	semail := r.Form.Get("email")
	key := r.Form.Get("key")
	if semail == "" || key == "" {
		writeBadRequest()
		return
	}
	user, err := db.GetUser(semail)
	if err != nil || user.Active == true || user.ActivateToken != key {
		writeBadRequest()
		return
	}
	user.Activate()
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	semail := r.FormValue("email")
	password := r.FormValue("password")
	user, err := db.NewUser(semail, password)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("%v", err)))
		return
	}
	if skipEmailVerification {
		user.Activate()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(user.Token))
	} else {
		email.SendRegistrationEmail(user)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Activation link was sent by email"))
	}
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	var pushData *db.PushData
	var err error
	var priority int
	var timestamp int64

	title := r.FormValue("title")
	body := r.FormValue("body")
	token := r.FormValue("token")
	stimestamp := r.FormValue("timestamp")
	spriority := r.FormValue("priority")
	uri := r.FormValue("url")

	// Parse priority, default to 1 - SavePushData will convert invalid
	// values to vaild ones
	if spriority != "" {
		priority, err = strconv.Atoi(spriority)
		if err != nil {
			priority = 1
		}
	}

	timestamp, err = strconv.ParseInt(stimestamp, 10, 64)
	if err != nil {
		timestamp = 0
	} else if timestamp < 0 {
		timestamp = 0
	}

	pushData, err = db.SavePushData(title, body, token, uri, timestamp, int64(priority))
	if err != nil {
		log.Printf("Something went wrong! (%v)", err)
		return
	}

	if pushData.Priority != 3 {
		// Send this to TCP client if any
		if send, ok := tcp.ClientFromPool(token); ok {
			data, err := pushData.ToJSON()
			if err != nil {
				// TODO: something went really wrong
			} else {
				select {
				case send <- string(data):
					if pushData.Priority == 2 {
						pushData.Sound = false
						pushData.Save()
					}
				default:
					// Buffer is full and tcp client is hanging on ping
					// message
				}
			}
		}
	}
	// NOTE: if we need pushData after this, we should reload it since it
	// might have been modified

	// If we made it here, push data was saved so lets notify GCM clients about that
	u, err := db.GetUserByToken(token)
	if err != nil {
		return
	}

	var regIds []string
	for _, c := range u.GCMClients {
		regIds = append(regIds, c.GCMId)
	}

	// If we dont have any GCM clients, don't even try to send data to them
	if len(regIds) > 0 {
		go utils.SendGcmPing(regIds)
	}
}

func poolHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data := ""
	token := r.FormValue("token")
	if db.TokenExists(token) {
		for _, push := range db.GetPushesForToken(token) {
			if push.Accessed {
				continue
			}
			tmp, err := push.ToJSON()
			push.SetAccessed()
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

func retrieveHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	semail := r.FormValue("email")
	password := r.FormValue("password")
	user, err := db.GetUser(semail)
	if err != nil || !user.ValidatePassword(password) || !user.Active {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(http.StatusText(http.StatusNotFound)))
	} else {
		w.Write([]byte(user.Token))
	}
}

func gcmRegisterHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	token := r.FormValue("token")
	gcmID := r.FormValue("gcmid")
	if gcmID == "" || token == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(http.StatusText(http.StatusBadRequest)))
	} else {
		_, err := db.RegisterGCMClient(gcmID, token)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(http.StatusText(http.StatusOK)))
		}
	}
}

func gcmUnregisterHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	gcmID := r.FormValue("gcmid")
	if gcmID == "" {
		return
	}
	g, err := db.GetGCMClient(gcmID)
	if err != nil {
		return
	}
	g.Delete()
}

func startTCP(addr string, config *tls.Config) {
	sock, err := tls.Listen("tcp", addr, config)
	if err != nil {
		log.Fatalf("startTCP: failed to bind socket (%v)\n", err)
		return
	}
	defer sock.Close()
	log.Printf("Listening for TCP connections on %s\n", addr)

	for {
		conn, err := sock.Accept()
		if err != nil {
			log.Printf("Failed to appect connection (%v)\n", err)
		} else {
			go tcp.HandleTCPClient(conn)
		}
	}
}

func main() {
	flag.Parse()
	config.GetConfig(*configFile)

	db.SetupDatabase()
	email.LoadConfig()
	utils.LoadConfig()

	logToTty, err := config.Config.Bool("log", "totty")
	logFile, err := config.Config.String("log", "file")
	host, err := config.Config.String("default", "host")
	port, err := config.Config.Int("default", "port")
	certPemFile, err := config.Config.String("ssl", "certpath")
	keyPemFile, err := config.Config.String("ssl", "keypath")
	skipEmailVerification, err = config.Config.Bool("registration", "skipEmailVerification")
	tcpHost, err := config.Config.String("tcp", "host")
	tcpPort, err := config.Config.Int("tcp", "port")

	if err != nil {
		log.Fatal(err)
	}

	cert, err := tls.LoadX509KeyPair(certPemFile, keyPemFile)
	if err != nil {
		log.Fatalf("Failed to load certificate key pair (%v)")
	}
	config := tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	httpHostPort = fmt.Sprintf("%s:%d", host, port)
	tcpHostPort := fmt.Sprintf("%s:%d", tcpHost, tcpPort)

	if !logToTty {
		f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	go startTCP(tcpHostPort, &config)

	http.HandleFunc("/register/", registerHandler)
	http.HandleFunc("/activate/", activateUserHandler)
	http.HandleFunc("/push/", pushHandler)
	http.HandleFunc("/pool/", poolHandler)
	http.HandleFunc("/retrieve/", retrieveHandler)
	http.HandleFunc("/gcm/", gcmRegisterHandler)
	http.HandleFunc("/ungcm/", gcmUnregisterHandler)

	if err := http.ListenAndServeTLS(httpHostPort, certPemFile, keyPemFile, nil); err != nil {
		panic(err)
	}
}
