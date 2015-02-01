package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

const (
	TokenMinLength = 8
	KeyMinLength   = 5
)

var host = flag.String("host", "localhost", "Address to bind")
var httpPort = flag.String("httpport", "8080", "Port to bind for pushing and http pooling")
var tcpPort = flag.String("poolport", "9098", "Port to bind for tcp pooling")
var logFile = flag.String("logfile", "/var/log/push-server.log", "File to save log data")
var logToTty = flag.Bool("logtty", false, "Output log to tty")

var db gorm.DB

var httpHostPort string
var tcpHostPort string

type HttpToken struct {
	Id        int64
	CreatedAt time.Time

	Token string `sql:unique`
	Key   string
}

// Register token for http pooling
func RegisterHttpToken(token, key string) (t *HttpToken, err error) {
	t = new(HttpToken)
	if db.Where("token = ?", token).First(t).RecordNotFound() {
		t = &HttpToken{
			Token: token,
			Key:   key,
		}
		if err = db.Save(t).Error; err != nil {
			return nil, err
		}
		return t, nil
	}
	return nil, fmt.Errorf("HttpToken already registered")
}

func (t *HttpToken) GetPushes() []*PushData {
	pushes := []*PushData{}
	db.Where("token = ?", t.Token).Find(&pushes)
	return pushes
}

// Queries db for tokens and returns one of token and key matches
func GetHttpToken(token, key string) (t *HttpToken, err error) {
	t = new(HttpToken)
	if db.Where("token = ?", token).First(t).RecordNotFound() {
		return nil, fmt.Errorf("Invalid key or token not found")
	}
	if key != t.Key {
		return nil, fmt.Errorf("Invalid key or token not found")
	}
	return t, nil
}

type PushData struct {
	Id        int64
	CreatedAt time.Time

	Title string
	Body  string
	Token string

	fetched chan bool `sql:"-"`
}

func SavePushData(title, body, token string) (p *PushData, err error) {
	p = &PushData{
		Title: title,
		Body:  body,
		Token: token,
	}
	if err = db.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (p *PushData) ToJson() ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func PushHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, err := SavePushData(r.FormValue("title"), r.FormValue("body"), r.FormValue("token"))
	if err != nil {
		log.Printf("Something went wrong! (%v)", err)
	}
	// TODO(vhakulinen): serve pushdata (on tcp and http timeouts)
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
		_, err := RegisterHttpToken(token, key)
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
	t, err := GetHttpToken(token, key)
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
	if err = http.ListenAndServe(httpHostPort, nil); err != nil {
		log.Fatal(err)
	}
}

func init() {
	var err error
	db, err = gorm.Open("sqlite3", "db.sqlite3")
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&PushData{})
	db.AutoMigrate(&HttpToken{})
}
