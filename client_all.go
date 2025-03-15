package ipc

import (
	"bufio"
	"errors"
	"fmt"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"io"
	"strings"
)

// DialAndHandshake - start the ipc client and return when connected or connection failed
// ipcName = is the name of the unix socket or named pipe that the client will try and connect to.
func DialAndHandshake(ipcName string, config *ClientConfig) (*Client, error) {
	c, err := DialAndHandshakeWithCallback(ipcName, config, nil)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// DialAndHandshakeWithCallback - start the ipc client and return when connected or connection failed
// ipcName = is the name of the unix socket or named pipe that the client will try and connect to.
func DialAndHandshakeWithCallback(ipcName string, config *ClientConfig, onConnectionStatusChanged func(ClientStatus)) (*Client, error) {
	c, err := createClient(ipcName, config)
	if err != nil {
		return nil, err
	}

	if onConnectionStatusChanged != nil {
		c.callback = onConnectionStatusChanged
	} else {
		c.callback = defaultCallbackOnStatusChange
	}
	go c.CallbackOnStatusChange(c.callback)

	dialToServer(c)
	err = c.StartProcessingMessages()
	if err != nil {
		return nil, err
	}

	log.Debugf("client: successfully waited, client is now '%s'", c.status)
	return c, nil
}

// DialAndHandshakeAsync - start the ipc client and return immediately while client connects to server in the background
// You have to call StartProcessingMessages() once you want to send/receive messages yourself then.
// ipcName = is the name of the unix socket or named pipe that the client will try and connect to.
func DialAndHandshakeAsync(ipcName string, config *ClientConfig, onConnectionStatusChanged func(ClientStatus)) (*Client, error) {
	c, err := createClient(ipcName, config)
	if err != nil {
		return nil, err
	}
	go c.CallbackOnStatusChange(onConnectionStatusChanged)
	go dialToServer(c)

	log.Debugf("client starting in background...")
	return c, nil
}

func (c *Client) StartProcessingMessages() error {
	if c.status != CConnected {
		return errors.New("client is not connected to server")
	}
	go c.clientReadDataFromConnectionToIncomingChannel()
	go c.clientWriteDataFromOutgoingChannelToConnection()
	return nil
}

func defaultCallbackOnStatusChange(status ClientStatus) {
	// nothing
}
func (c *Client) CallbackOnStatusChange(onConnected func(ClientStatus)) {
	for {
		status := <-c.statusChannel
		log.Statusf("client: client status is now '%s'", c.status)
		if onConnected != nil {
			onConnected(status)
		}
	}
}

func dialToServer(c *Client) {
	log.Debugln("client: dialToServer")
	c.status = CConnecting
	c.statusChannel <- CConnecting

	err := c.clientDialAndHandshakeToServer()
	if err != nil {
		log.Warn(err)
		c.status = CError
		c.statusChannel <- CError
		return
	}
	c.status = CConnected
	log.Debugln("client BEFORE connected <- true")
	c.statusChannel <- CConnected
	log.Debugln("client connected <- true")
}

func (c *Client) clientReadDataFromConnectionToIncomingChannel() {
	bLen := make([]byte, 4)

	for {
		res := c.readData(bLen)
		if !res {
			break
		}

		mLen := bytesToInt(bLen)
		msg := make([]byte, mLen)
		res = c.readData(msg)
		if !res {
			break
		}

		var err error
		if c.conf.Encryption {
			msg, err = decrypt(*c.enc.cipher, msg)
			if err != nil {
				break
			}
		}
		msgTypeInt := bytesToInt(msg[:4])
		msgData := msg[4:]
		if msgTypeInt < 0 {
			// TODO some func to call for internal messages from the other side
		} else {
			c.incoming <- NewMessage(MsgType(msgTypeInt), msgData)
		}
	}
}

func (c *Client) readData(buff []byte) bool {
	_, err := io.ReadFull(c.conn, buff)
	if err != nil {
		if strings.Contains(err.Error(), "EOF") { // the connection has been closed by the client.
			c.conn.Close()
			if c.status != CClosing {
				go c.reconnect()
			}

			return false
		}

		if c.status == CClosing {
			c.status = CClosed
			c.statusChannel <- CClosed

			return false
		}

		// other serverReadDataFromConnectionToIncomingChannel error
		return false
	}

	return true
}

func (c *Client) reconnect() {
	c.ClearConnectionStatus()
	c.status = CReConnecting
	c.statusChannel <- CReConnecting
	err := c.clientDialAndHandshakeToServer() // connect to the pipe
	if err != nil {
		if err.Error() == "client timed out trying to connect" {
			c.status = CTimeout
			c.statusChannel <- CTimeout
		} else {
			c.status = CError
			c.statusChannel <- CError
		}
		return
	}

	c.status = CConnected
	c.statusChannel <- CConnected

	go c.clientReadDataFromConnectionToIncomingChannel()
}

// Read - blocking function that receives messages
// if MsgType is a negative number it's an internal message
func (c *Client) Read() (*Message, error) {
	m, ok := <-c.incoming
	if !ok {
		return nil, errors.New("client the received channel has been closed")
	}

	if m.Err != nil {
		close(c.incoming)
		close(c.outgoing)

		return nil, m.Err
	}

	return m, nil
}

// Write - writes a  message to the ipc connection.
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (c *Client) Write(msgType MsgType, message []byte) error {
	if msgType <= 0 {
		return errors.New(fmt.Sprintf("client Write: cannot because message type %d is reserved (0 or below)", msgType))
	}

	if c.status != CConnected {
		return errors.New(fmt.Sprintf("client Write: cannot because client.status is: %s", c.status.String()))
	}

	msgLength := len(message)
	if msgLength > c.conf.MaxMsgSize {
		return errors.New("client Write: cannot because message exceeds maximum message length")
	}

	c.outgoing <- NewMessage(msgType, message)

	return nil
}

// clientWriteDataFromOutgoingChannelToConnection a message to Client.outgoing channel
// eventually a message is structured as follows: lengthOfMsgTypePlusMessage + MsgType + Message
func (c *Client) clientWriteDataFromOutgoingChannelToConnection() {
	var err error
	for {
		msg, ok := <-c.outgoing
		if !ok {
			break
		}

		// eventually sending: MsgType + Message
		toSend := msg.MsgType.toBytes()
		toSend = append(toSend, msg.Data...)

		if c.conf.Encryption {
			toSend, err = encrypt(*c.enc.cipher, toSend)
			if err != nil {
				log.Debugln("client error encrypting data", err)
				continue
			}
		}

		writer := bufio.NewWriter(c.conn)
		writer.Write(intToBytes(len(toSend)))
		writer.Write(toSend)
		err = writer.Flush()
		if err != nil {
			log.Debugln("client error flushing data", err)
			continue
		}
	}
}

// Status StatusCode - returns the current connection status
func (c *Client) Status() ClientStatus {
	return c.status
}

// Close - closes the connection
func (c *Client) Close() {
	c.status = CClosing
	c.statusChannel <- CClosing
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) ClearConnectionStatus() {
	select {
	case <-c.statusChannel:
		// silently emptying "connected" channel (if there is something in it)
	default:
		// there wasn't anything in it anyway
	}
	c.status = CNotConnected
	c.statusChannel <- CNotConnected
}

func createClient(ipcName string, config *ClientConfig) (*Client, error) {
	err := ipcNameValidate(ipcName)
	if err != nil {
		return nil, err
	}

	c := &Client{
		Name:          ipcName,
		status:        CNotConnected,
		statusChannel: make(chan ClientStatus),
		incoming:      make(chan *Message),
		outgoing:      make(chan *Message),
	}

	if config == nil {
		c.conf = DefaultClientConfig
	} else {
		c.conf = *config
	}

	if c.conf.Timeout < 0 {
		c.conf.Timeout = DefaultClientConfig.Timeout
	}
	if c.conf.RetryTimer <= 0 {
		c.conf.RetryTimer = DefaultClientConfig.RetryTimer
	}
	if c.conf.SocketBasePath == "" {
		c.conf.SocketBasePath = DefaultClientConfig.SocketBasePath
	}
	return c, nil
}
