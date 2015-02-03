package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/vhakulinen/push-server/pushserv"
)

const (
	TokenMinLength = 8
	KeyMinLength   = 5

	// In minutes
	DeleteLoopInterval = 5
	HttpTokenExpires   = 10
	PushDataExpires    = 10
)

var host = flag.String("host", "localhost", "Address to bind")
var httpPort = flag.String("httpport", "8080", "Port to bind for pushing and http pooling")
var tcpPort = flag.String("poolport", "9098", "Port to bind for tcp pooling")
var logFile = flag.String("logfile", "/var/log/push-server.log", "File to save log data")
var logToTty = flag.Bool("logtty", false, "Output log to tty")
var certPemFile = flag.String("cert", "cert.pem", "Certificate pem file")
var keyPemFile = flag.String("key", "key.pem", "Key pem file")

var httpHostPort string
var tcpHostPort string

func PushHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, err := pushserv.SavePushData(r.FormValue("title"), r.FormValue("body"), r.FormValue("token"))
	if err != nil {
		log.Printf("Something went wrong! (%v)", err)
	}
}

func TokenHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	token := r.FormValue("token")
	key := r.FormValue("key")
	// Use register variable to register new token
	if len(token) < TokenMinLength && len(key) < KeyMinLength {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(fmt.Sprintf("Token min length: %d\nKey min length: %d", TokenMinLength, KeyMinLength)))
	} else {
		_, err := pushserv.RegisterHttpToken(token, key)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Error (%s)", err)))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Token registered"))
		}
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

// Runs in its own goroutine. Deletes expired http tokens and data pushes
// on that token
func DeleteExpiredHttpTokensAndPushDatas() {
	for {
		tokens := pushserv.GetAllTokens()
		for _, token := range tokens {
			if time.Since(token.AccessedAt) > time.Minute*HttpTokenExpires {
				token.Delete()
			}
		}
		pushdatas := pushserv.GetAllPushDatas()
		for _, data := range pushdatas {
			if time.Since(data.CreatedAt) > time.Minute*PushDataExpires {
				data.Delete()
			}
		}
		time.Sleep(time.Minute * DeleteLoopInterval)
	}
}

func main() {
	flag.Parse()
	httpHostPort = fmt.Sprintf("%s:%s", *host, *httpPort)
	tcpHostPort = fmt.Sprintf("%s:%s", *host, *tcpPort)

	if !*logToTty {
		f, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	tcpPoolSock, err := net.Listen("tcp", tcpHostPort)
	if err != nil {
		log.Fatalf("Couldn't bind %s for push socket (%s)\n", tcpHostPort, err)
		return
	}
	defer tcpPoolSock.Close()

	go DeleteExpiredHttpTokensAndPushDatas()

	// TCP pooling
	go func() {
		for {
			conn, err := tcpPoolSock.Accept()
			if err != nil {
				log.Printf("Failed to accept connection on pool (%s)\n", err)
			} else {
				// TODO(vhakulinen): Implement tcp client
				conn.Close()
			}
		}
	}()

	http.HandleFunc("/push/", PushHandler)
	http.HandleFunc("/pool/", PoolHandler)
	http.HandleFunc("/token/", TokenHandler)
	if err = http.ListenAndServeTLS(httpHostPort, *certPemFile, *keyPemFile, nil); err != nil {
		log.Fatal(err)
	}
}
