package tcp

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/vhakulinen/push-server/db"
)

const (
	// Seconds to wait for the token message
	tokenReadDeadLine = 60
	// The token's length is 36, but let's be ready for some witespace just in case
	tokenMessageLen = 40
)

var tcpPool map[string]chan string

func ClientFromPool(token string) (chan string, bool) {
	c, ok := tcpPool[token]
	return c, ok
}

func HandleTCPClient(conn net.Conn) {
	var token string
	var registeredToBool = false
	defer func() {
		conn.Close()
		if _, ok := tcpPool[token]; ok && registeredToBool {
			delete(tcpPool, token)
		}
	}()
	var sendChan chan string
	buf := make([]byte, tokenMessageLen)

	// Dont wait forever for the first message
	conn.SetReadDeadline(time.Now().Add(time.Second * tokenReadDeadLine))
	count, err := conn.Read(buf)
	if err != nil {
		//TODO: logging
		return
	}

	token = strings.TrimSpace(string(buf[:count]))
	if _, err = db.GetHttpToken(fmt.Sprintf("%s", token)); err != nil {
		conn.Write([]byte("Token not found!"))
		return
	}
	if _, ok := tcpPool[token]; ok {
		conn.Write([]byte("TCP client already listening for this token!"))
		return
	}

	sendChan = make(chan string)
	tcpPool[token] = sendChan
	registeredToBool = true

	// No need for deadline anymore
	conn.SetReadDeadline(time.Time{})

	// If we read _anything_ from client anymore, close it and close
	// the sendChan so we can exit from the for loop
	go func() {
		conn.Read(make([]byte, 1))
		close(sendChan)
	}()

	for {
		data, ok := <-sendChan
		if ok {
			_, err = conn.Write([]byte(data))
			if err != nil {
				// TODO: Logging
				close(sendChan)
				break
			}
		} else {
			break
		}
	}
}

func init() {
	tcpPool = make(map[string]chan string)
}
