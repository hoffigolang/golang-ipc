package ipc

import (
	"bufio"
	"errors"
	"fmt"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"io"
	"time"
)

// StartServer - starts the ipc server.
//
// ipcName - is the name of the unix socket or named pipe that will be created, the client needs to use the same name
func StartServer(ipcName string, config *ServerConfig) (*Server, error) {
	s, err := createServer(ipcName, config)
	if err != nil {
		return nil, err
	}

	s.callback = consumeServerStatusChanges
	go s.CallbackOnStatusChange(s.callback)
	err = s.serverRun()
	go s.acceptClientConnectionsLoop()

	return s, err
}

func consumeServerStatusChanges(status ServerStatus) {
	// nothing
}
func (s *Server) CallbackOnStatusChange(onConnected func(ServerStatus)) {
	for {
		status := <-s.statusChannel
		log.Statusf("server: server status is now '%s'", s.status)
		if onConnected != nil {
			onConnected(status)
		}
	}
}

func (s *Server) acceptClientConnectionsLoop() {
	for {
		conn, err := s.listen.Accept()
		if err != nil {
			break
		} else {
			log.Debugln("server conn: client wants to connect... initiating handshake...")
		}

		if s.status == SListening || s.status == SDisconnected {
			s.conn = conn

			err = s.serverHandshake()
			if err != nil {
				s.status = SError
				s.statusChannel <- SError
				s.listen.Close()
				s.conn.Close()
			} else {
				s.status = SConnected
				s.statusChannel <- SConnected
				s.clientConnectionCount += 1
				log.Debugln("server (a) client clientConnected <- true")
				go s.serverReadDataFromConnectionToIncomingChannel()
				go s.serverWriteDataFromOutgoingChannelToConnection()
			}
		}
	}
}

func (s *Server) serverReadDataFromConnectionToIncomingChannel() {
	bLen := make([]byte, 4)

	for {
		res := s.readDataFromConnection(bLen)
		if !res {
			s.conn.Close()
			break
		}

		mLen := bytesToInt(bLen)
		msg := make([]byte, mLen)
		res = s.readDataFromConnection(msg)
		if !res {
			s.conn.Close()
			break
		}

		var err error
		if s.conf.Encryption {
			msg, err = decrypt(*s.enc.cipher, msg)
			if err != nil {
				s.incoming <- NewIpcErrorMessage(err)
				continue
			}
		}
		msgTypeInt := bytesToInt(msg[:4])
		msgData := msg[4:]
		if msgTypeInt < 0 {
			// TODO some func to call for internal messages from the other side
			log.Debugf("server.serverReadDataFromConnectionToIncomingChannel() msgFinal HandshakeOk: %d", bytesToInt(msg))
		} else {
			s.incoming <- NewMessage(MsgType(msgTypeInt), msgData)
		}
	}
}

func (s *Server) readDataFromConnection(buff []byte) bool {
	_, err := io.ReadFull(s.conn, buff)
	if err != nil {
		if s.status == SClosing {
			s.status = SClosed
			s.statusChannel <- SClosed
			return false
		}

		if err == io.EOF {
			s.status = SDisconnected
			s.statusChannel <- SDisconnected
			return false
		}
	}

	return true
}

//func (s *Server) reConnect() {
//    s.status = ReConnecting
//    s.received <- &Message{Status: s.status.String(), MsgType: Internal}
//
//    err := s.connectionTimer()
//    if err != nil {
//        s.status = Timeout
//        s.received <- &Message{Status: s.status.String(), MsgType: Internal}
//
//        s.received <- &Message{err: err, MsgType: -2}
//    }
//}

// Read - blocking function, reads each message recieved
// if MsgType is a negative number its an internal message
func (s *Server) Read() (*Message, error) {
	msg, ok := <-s.incoming
	if !ok {
		return nil, errors.New("server the received channel has been closed")
	}

	if msg.Err != nil {
		return nil, msg.Err
	}

	return msg, nil
}

// Write - writes a message to the ipc connection
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (s *Server) Write(msgType MsgType, message []byte) error {
	if msgType <= 0 {
		return errors.New(fmt.Sprintf("server message type %d is reserved (0 or below)", msgType))
	}

	msgLength := len(message)

	if msgLength > s.conf.MaxMsgSize {
		return errors.New("server message exceeds maximum message length")
	}

	if s.status == SConnected {
		s.outgoing <- NewMessage(msgType, message)
	} else {
		return errors.New(s.status.String())
	}

	return nil
}

func (s *Server) serverWriteDataFromOutgoingChannelToConnection() {
	var err error
	for {
		msg, ok := <-s.outgoing
		if !ok {
			break
		}

		toSend := msg.MsgType.toBytes()
		writer := bufio.NewWriter(s.conn)

		if s.conf.Encryption {
			toSend = append(toSend, msg.Data...)
			toSend, err = encrypt(*s.enc.cipher, toSend)
			if err != nil {
				log.Debugln("server error encrypting data", err)

				continue
			}
		} else {
			toSend = append(toSend, msg.Data...)
		}

		writer.Write(intToBytes(len(toSend)))
		writer.Write(toSend)
		err = writer.Flush()
		if err != nil {
			log.Debugln("server error flushing data", err)

			continue
		}

		time.Sleep(10_000 * time.Nanosecond)
	}
}

// Status - returns the current connection status
func (s *Server) Status() ServerStatus {
	return s.status
}

// Close - closes the connection
func (s *Server) Close() {
	s.status = SClosing
	s.statusChannel <- SClosing

	if s.listen != nil {
		s.listen.Close()
	}

	if s.conn != nil {
		s.conn.Close()
	}

	if s.incoming != nil {
		s.incoming <- NewIpcConnectionErrorMessage(errors.New("server has already closed the connection"))

		close(s.incoming)
	}

	if s.outgoing != nil {
		close(s.outgoing)
	}
}

func createServer(ipcName string, config *ServerConfig) (*Server, error) {
	err := ipcNameValidate(ipcName)
	if err != nil {
		return nil, err
	}

	s := &Server{
		Name:                  ipcName,
		status:                SNotConnected,
		clientConnectionCount: 0,
		statusChannel:         make(chan ServerStatus),
		incoming:              make(chan *Message),
		outgoing:              make(chan *Message),
	}

	if config == nil {
		s.conf = DefaultServerConfig
	} else {
		s.conf = *config
	}

	if s.conf.Timeout < 0 {
		s.conf.Timeout = DefaultServerConfig.Timeout
	}
	if s.conf.MaxMsgSize < minMsgSize {
		s.conf.MaxMsgSize = DefaultServerConfig.MaxMsgSize
	}
	if s.conf.SocketBasePath == "" {
		s.conf.SocketBasePath = DefaultServerConfig.SocketBasePath
	}
	return s, nil
}
