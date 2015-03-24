package tcp

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/vhakulinen/push-server/db"
)

const (
	// Seconds to wait for the token message
	tokenReadDeadLine = 60
	// The token's length is 36, so lets only read it
	tokenMessageLen = 36
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

var ClientFromPool = func(token string) (chan<- string, bool) {
	c, ok := peers.Get(token)
	return c, ok
}

func HandleTCPClient(conn net.Conn) {
	var token string
	var sendChan = make(chan string)
	var quitChan = make(chan bool)
	defer func() {
		conn.Close()
		if token != "" {
			peers.Remove(token)
		}
		close(sendChan)
		close(quitChan)
	}()
	// Dont wait forever for the first message
	conn.SetReadDeadline(time.Now().Add(time.Second * tokenReadDeadLine))

	buf := make([]byte, tokenMessageLen)
	count, err := io.ReadFull(conn, buf)
	if err != nil {
		//TODO: logging
		if count < tokenMessageLen {
			conn.Write([]byte("Timeout"))
		}
		return
	}

	token = string(buf)
	if !db.TokenExists(fmt.Sprintf("%s", token)) {
		conn.Write([]byte("Token not found!"))
		return
	}
	if err = peers.Set(token, sendChan); err != nil {
		conn.Write([]byte("Client already listening for this token"))
		return
	}

	// No need for deadline anymore
	conn.SetReadDeadline(time.Time{})

	go func() {
		conn.Read(make([]byte, 1))
		quitChan <- true
	}()

	for {
		select {
		case data, ok := <-sendChan:
			if ok {
				conn.Write([]byte(data + "\n"))
			} else {
				return
			}
		case <-quitChan:
			return
		}
	}
}

func init() {
	peers = tcpPool{
		m: make(map[string]chan<- string),
	}
}
