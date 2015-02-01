package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

const (
	PushReadTimeout = 156
	BufSize         = 1024
)

var host = flag.String("host", "localhost", "Address to bind")
var pushPort = flag.String("pushPort", "9099", "Port to bind for pushing")
var poolPort = flag.String("poolPort", "9098", "Port to bind for pooling")
var logfile = flag.String("logfile", "/var/log/push-server.log", "File to save log data")
var logToTty = flag.Bool("logtty", false, "Output log to tty")

var pushHostPort string
var poolHostPort string

// TODO: Add safe read/write functions to clientPool
var clientPool map[string][]poolClient

type PushData struct {
	Title string
	Body  string
	Token string
}

type FirstMessage struct {
	Token string
}

type poolClient struct {
	conn     net.Conn
	addr     net.Addr
	sendchan chan []byte
	token    string
}

func NewPoolClient(conn net.Conn) *poolClient {
	p := &poolClient{
		conn:     conn,
		addr:     conn.RemoteAddr(),
		sendchan: make(chan []byte),
	}
	return p
}

func (p *poolClient) removeFromPool() {
	if slice, ok := clientPool[p.token]; ok {
		count := 0
		for i, client := range slice {
			if client == *p {
				break
			}
			i++
		}
		slice = append(slice[:count], slice[count+1:]...)
		clientPool[p.token] = slice
		log.Printf("Removed client from pool\n")
	}
}

func (p *poolClient) Send(v *PushData) {
	defer func() {
		if x := recover(); x != nil {
			log.Printf("Unable to send: %s\n", x)
		}
	}()
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Failed to parse data to be sended to client (%s)\n", err)
		return
	}
	p.sendchan <- data
}

// Gorutine for pool client
func (p *poolClient) Listen() {
	defer p.conn.Close()
	go func() {
		for {
			buf := make([]byte, BufSize)
			count, err := p.conn.Read(buf)
			if err != nil {
				log.Printf("Client exited\n")
				close(p.sendchan)
				return
			} else {
				log.Printf("Received data from client %s\n", buf)
				var first FirstMessage
				if err := json.Unmarshal(buf[0:count], &first); err != nil {
					log.Printf("Failed to parse data from client (%s)\n", err)
				} else {
					_, ok := clientPool[first.Token]
					if ok {
						clientPool[first.Token] = append(clientPool[first.Token], *p)
					} else {
						clientPool[first.Token] = []poolClient{*p}
					}
					p.token = first.Token
					log.Printf("Added client to pool with token %s\n", p.token)
				}
			}
		}
	}()
	for {
		data, ok := <-p.sendchan
		if ok {
			_, err := p.conn.Write(data)
			if err != nil {
				log.Printf("Failed to write data to client (%s)", err)
				log.Printf("Closing client")
				p.conn.Close()
			}
		} else {
			log.Printf("Channel closed, exiting client\n")
			p.removeFromPool()
			return
		}
	}
}

func pushHandle(conn net.Conn) {
	defer conn.Close()
	addr := conn.RemoteAddr()
	buf := make([]byte, BufSize)
	conn.SetReadDeadline(time.Now().Add(time.Second * PushReadTimeout))
	count, err := conn.Read(buf)
	if err != nil {
		if err == io.EOF {
			log.Printf("Client closed (%s)\n", err)
		} else {
			log.Printf("Failed to receive data from %s (%s)\n", addr.String(), err)
		}
		log.Printf("Closing client\n")
		return
	}
	var v PushData
	if err := json.Unmarshal(buf[0:count], &v); err != nil {
		log.Printf("Failed to parse data %s (%s)\n", buf, err)
	} else {
		log.Printf("Received data Title: %s Body: %s Token: %s from %s\n",
			v.Title, v.Body, v.Token, conn.RemoteAddr())
		if clientSlice, ok := clientPool[v.Token]; ok {
			for _, client := range clientSlice {
				client.Send(&v)
			}
		}
	}
}

func main() {
	flag.Parse()
	pushHostPort = fmt.Sprintf("%s:%s", *host, *pushPort)
	poolHostPort = fmt.Sprintf("%s:%s", *host, *poolPort)

	if !*logToTty {
		f, err := os.OpenFile(*logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	pushsock, err := net.Listen("tcp", pushHostPort)
	if err != nil {
		log.Fatalf("Couldn't bind %s for push socket (%s)\n", pushHostPort, err)
		return
	}
	defer pushsock.Close()

	poolsock, err := net.Listen("tcp", poolHostPort)
	if err != nil {
		log.Fatalf("Couldn't bind %s for pool socket (%s)\n", poolHostPort, err)
		return
	}
	defer poolsock.Close()

	log.Printf("Listening connections on %s and on %s\n", pushHostPort, poolHostPort)

	// pushing
	go func() {
		for {
			conn, err := pushsock.Accept()
			if err != nil {
				log.Print("Failed to accept connection on push (%s)\n", err)
			} else {
				go pushHandle(conn)
			}
		}
	}()

	// pooling
	for {
		conn, err := poolsock.Accept()
		if err != nil {
			log.Printf("Failed to accept connection on pool (%s)\n", err)
		} else {
			p := NewPoolClient(conn)
			go p.Listen()
		}
	}
}

func init() {
	clientPool = make(map[string][]poolClient)
}
