//go:build windows
// +build windows

package ipc

import (
	"errors"
	"github.com/Microsoft/go-winio"
	log "github.com/hoffigolang/golang-ipc/ipclogging"
	"path/filepath"
	"strings"
	"time"
)

var defaultSocketBasePath = `\\.\pipe\`

// Create the named pipe (if it doesn't already exist) and start listening for a client to connect.
// when a client connects and connection is accepted the serverReadDataFromConnectionToIncomingChannel function is called on a go routine.
func (s *Server) serverRun() error {
	socketPath := filepath.Join(s.conf.SocketBasePath, s.Name)
	var config *winio.PipeConfig

	if s.conf.UnmaskPermissions {
		config = &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;AU)"}
	}

	listen, err := winio.ListenPipe(socketPath, config)
	if err != nil {
		return err
	}
	s.listen = listen
	s.status = SListening
	s.statusChannel <- SListening

	log.Debugln("server ok connected to namedPipe ... waiting for clients to connect...")
	return nil
}

// clientDialAndHandshakeToServer - attempts to connect to a named pipe created by the server
func (c *Client) clientDialAndHandshakeToServer() error {
	socketPath := filepath.Join(c.conf.SocketBasePath, c.Name)
	startTime := time.Now()

	for {
		if c.conf.Timeout != 0 {
			if time.Since(startTime) > c.conf.Timeout {
				c.status = CError
				c.statusChannel <- CError
				return errors.New("client timed out trying to connect")
			}
		}

		namedPipe, err := winio.DialPipe(socketPath, nil)
		if err != nil {
			if strings.Contains(err.Error(), "the system cannot find the file specified.") {
				// waiting for the server to come up
			} else {
				return err
			}
		} else {
			c.conn = namedPipe
			log.Debugln("client connected to server namedPipe ... now waiting for server handshake")
			err = c.clientDoPassiveHandshake()
			if err != nil {
				return err
			}

			return nil
		}

		time.Sleep(c.conf.RetryTimer * time.Second)
	}
}
