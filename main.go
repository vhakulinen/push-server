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

func ActivateUserHandler(w http.ResponseWriter, r *http.Request) {
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

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
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

func PushHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var pushData *db.PushData
	var err error

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
		pushData, err = db.SavePushData(title, body, token, timestamp)
		if err != nil {
			log.Printf("Something went wrong! (%v)", err)
			return
		}
	} else {
		pushData, err = db.SavePushDataMinimal(title, body, token)
		if err != nil {
			log.Printf("Something went wrong! (%v)", err)
			return
		}
	}

	// Send this to TCP client if any
	if send, ok := tcp.ClientFromPool(token); ok {
		data, err := pushData.ToJson()
		if err != nil {
			// TODO: something went really wrong
		} else {
			send <- string(data)
		}
	}

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

func PoolHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data := ""
	token := r.FormValue("token")
	if db.TokenExists(token) {
		for _, push := range db.GetPushesForToken(token) {
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

func GCMRegisterHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	token := r.FormValue("token")
	gcmId := r.FormValue("gcmid")
	if gcmId == "" || token == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(http.StatusText(http.StatusBadRequest)))
	} else {
		_, err := db.RegisterGCMClient(gcmId, token)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(http.StatusText(http.StatusOK)))
		}
	}
}

func startTcp(addr string, config *tls.Config) {
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

	go startTcp(tcpHostPort, &config)

	http.HandleFunc("/register/", RegisterHandler)
	http.HandleFunc("/activate/", ActivateUserHandler)
	http.HandleFunc("/push/", PushHandler)
	http.HandleFunc("/pool/", PoolHandler)
	http.HandleFunc("/retrieve/", RetrieveHandler)
	http.HandleFunc("/gcm/", GCMRegisterHandler)

	if err := http.ListenAndServeTLS(httpHostPort, certPemFile, keyPemFile, nil); err != nil {
		panic(err)
	}
}
