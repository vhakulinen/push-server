package tcp

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/vhakulinen/push-server/db"
	"github.com/vhakulinen/push-server/utils"
)

const (
	// Seconds to wait for the token message
	tokenReadDeadLine = 60
	// The token's length is 36, so lets only read it
	tokenMessageLen = 36

	pingTimeout  = 20
	pingInterval = 120

	chanBufferSize = 100
)

type tcpPool struct {
	m  map[string]chan<- string
	mu sync.RWMutex // protects m
}

func (t *tcpPool) Get(token string) (chan<- string, bool) {
	t.mu.RLock()
	c, ok := t.m[token]
	t.mu.RUnlock()
	return c, ok
}

func (t *tcpPool) Set(token string, c chan<- string) error {
	// Check whether token is already in pool
	_, ok := t.Get(token)
	if ok {
		return fmt.Errorf("Token already in map")
	}

	t.mu.Lock()
	t.m[token] = c
	t.mu.Unlock()
	return nil
}

func (t *tcpPool) Remove(token string) error {
	if _, ok := t.Get(token); ok {
		t.mu.Lock()
		delete(t.m, token)
		t.mu.Unlock()
		return nil
	}
	return fmt.Errorf("Token not in map")
}

var peers tcpPool

// ClientFromPool is link to map where TCP client send channels are kept
var ClientFromPool = func(token string) (chan<- string, bool) {
	c, ok := peers.Get(token)
	return c, ok
}

// HandleTCPClient handles new TCP client connections
func HandleTCPClient(conn net.Conn) {
	var token string
	var sendChan = make(chan string, chanBufferSize)
	defer func() {
		conn.Close()
		if token != "" {
			peers.Remove(token)
		}
		close(sendChan)
	}()
	// Dont wait forever for the first message
	conn.SetReadDeadline(time.Now().Add(time.Second * tokenReadDeadLine))

	buf := make([]byte, tokenMessageLen)
	count, err := io.ReadFull(conn, buf)
	if err != nil {
		//TODO: logging
		if count < tokenMessageLen {
			conn.Write([]byte("Timeout\n"))
		}
		return
	}

	if !db.TokenExists(fmt.Sprintf("%s", string(buf))) {
		conn.Write([]byte("Token not found!\n"))
		return
	}
	if err = peers.Set(string(buf), sendChan); err != nil {
		conn.Write([]byte("Client already listening for this token\n"))
		return
	}
	token = string(buf)

	c := time.After(time.Second * pingInterval)
	for {
		select {
		case data, ok := <-sendChan:
			if ok {
				_, err := conn.Write([]byte(data + "\n"))
				if err != nil {
					return
				}
			} else {
				return
			}
		case <-c:
			// Send ping
			msg := utils.RandomString(5)
			i, err := conn.Write([]byte(fmt.Sprintf(":PING %s\n", msg)))
			if i != 12 || err != nil {
				log.Printf("%v", err)
				return
			}

			// Read pong
			conn.SetReadDeadline(time.Now().Add(time.Second * pingTimeout))
			buf := make([]byte, 12)
			i, err = conn.Read(buf)

			// Check it
			if i != 12 || err != nil || string(buf) != fmt.Sprintf(":PONG %s\n", msg) {
				return
			}
			c = time.After(time.Second * pingInterval)
		}
	}
}

func init() {
	peers = tcpPool{
		m: make(map[string]chan<- string),
	}
}
