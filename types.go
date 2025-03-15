package ipc

import (
	"crypto/cipher"
	"net"
	"time"
)

// Server - holds the details of the server connection & config.
type Server struct {
	Name                  string
	listen                net.Listener // listener for connections
	conn                  net.Conn     // socket/namedPipe connection to a client
	status                ServerStatus
	statusChannel         chan ServerStatus // reacting to Server status changes
	clientConnectionCount int
	callback              func(ServerStatus)
	incoming              chan *Message
	outgoing              chan *Message
	enc                   *encryption
	conf                  ServerConfig
}

// Client - holds the details of the client connection and config.
type Client struct {
	Name          string
	conn          net.Conn // socket/namedPipe connection to server
	status        ClientStatus
	statusChannel chan ClientStatus // reacting to Client status changes
	callback      func(ClientStatus)
	incoming      chan *Message
	outgoing      chan *Message
	enc           *encryption
	conf          ClientConfig
}

// Message - contains the received message or to send message
type Message struct {
	Err     error      // details of any error
	IpcType IpcMsgType // if not 0 this is an Ipc specific message, not an ordinary message
	MsgType MsgType    // 0 = reserved , <0 is an internal message (disconnection or error etc), all "normal" messages received will be > 0
	Status  Status     // the connection status (mostly for internal IpcMsgType messages)
	Data    []byte     // message data
}

type Status int

const (
	None               = 0
	ServerNotConnected = Status(SNotConnected)
	ServerListening    = Status(SListening)
	ServerConnecting   = Status(SConnecting)
	ServerConnected    = Status(SConnected)
	ServerReConnecting = Status(SReConnecting)
	ServerClosing      = Status(SClosing)
	ServerClosed       = Status(SClosed)
	ServerError        = Status(SError)
	ServerTimeout      = Status(STimeout)
	ServerDisconnected = Status(SDisconnected)

	ClientNotConnected = Status(CNotConnected)
	ClientListening    = Status(CListening)
	ClientConnecting   = Status(CConnecting)
	ClientConnected    = Status(CConnected)
	ClientReConnecting = Status(CReConnecting)
	ClientClosing      = Status(CClosing)
	ClientClosed       = Status(CClosed)
	ClientError        = Status(CError)
	ClientTimeout      = Status(CTimeout)
	ClientDisconnected = Status(CDisconnected)
)

type ServerStatus int

const (
	SNotConnected ServerStatus = iota + 10 // 10
	SListening                             // 11
	SConnecting                            // 12
	SConnected                             // 13
	SReConnecting                          // 14
	SClosing                               // 16
	SClosed                                // 15
	SError                                 // 17
	STimeout                               // 18
	SDisconnected                          // 19
)

type ClientStatus int

const (
	CNotConnected ClientStatus = iota + 20 // 20
	CListening                             // 21
	CConnecting                            // 22
	CConnected                             // 23
	CReConnecting                          // 24
	CClosing                               // 26
	CClosed                                // 25
	CError                                 // 27
	CTimeout                               // 28
	CDisconnected                          // 29
)

func (status Status) String() string {
	return StatusString(status)
}
func (status ClientStatus) String() string {
	return StatusString(Status(status))
}
func (status ServerStatus) String() string {
	return StatusString(Status(status))
}
func StatusString(status Status) string {
	switch status {
	case None:
		return "None"
	case ServerNotConnected:
		return "ServerNotConnected"
	case ServerListening:
		return "ServerListening"
	case ServerConnecting:
		return "ServerConnecting"
	case ServerConnected:
		return "ServerConnected"
	case ServerReConnecting:
		return "ServerReConnecting"
	case ServerClosing:
		return "ServerClosing"
	case ServerClosed:
		return "ServerClosed"
	case ServerError:
		return "ServerError"
	case ServerTimeout:
		return "ServerTimeout"
	case ServerDisconnected:
		return "ServerDisconnected"

	case ClientNotConnected:
		return "ClientNotConnected"
	case ClientListening:
		return "ClientListening"
	case ClientConnecting:
		return "ClientConnecting"
	case ClientConnected:
		return "ClientConnected"
	case ClientReConnecting:
		return "ClientReConnecting"
	case ClientClosing:
		return "ClientClosing"
	case ClientClosed:
		return "ClientClosed"
	case ClientError:
		return "ClientError"
	case ClientTimeout:
		return "ClientTimeout"
	case ClientDisconnected:
		return "ClientDisconnected"
	default:
		return "Status not found"
	}
}

type MsgType int

const (
	Error  = iota + 1 // 1
	String            // 2
	Int               // 3
	Float             // 4
	Struct            // 5
	Custom            // 6
)

func (mt MsgType) String() string {
	return MsgTypeString(mt)
}
func MsgTypeString(mt MsgType) string {
	switch mt {
	case String:
		return "String"
	case Int:
		return "Int"
	case Float:
		return "Float"
	case Struct:
		return "Struct"
	case Custom:
		return "Custom"
	default:
		return "<Unknown>"
	}
}

type IpcMsgType int

const (
	ConnectionError IpcMsgType = iota - 5 // -5
	OtherError                            // -4
	IpcLocalMsg                           // -3
	IpcRemoteMsg                          // -2
	IpcHandshake                          // -1
	NoIpcMsg                              // 0 meaning this is a "normal" message
)

func (imt IpcMsgType) String() string {
	return IpcMsgTypeString(imt)
}
func IpcMsgTypeString(imt IpcMsgType) string {
	switch imt {
	case ConnectionError:
		return "ConnectionError"
	case IpcLocalMsg:
		return "IpcLocalMsg"
	case IpcRemoteMsg:
		return "IpcRemoteMsg"
	case IpcHandshake:
		return "IpcHandshake"
	case NoIpcMsg:
		return "NoIpcMsg"
	default:
		return "<Unknown>"
	}
}

type HandshakeResult byte

const (
	HandshakeOk                  HandshakeResult = iota // 0
	IpcVersionMismatch                                  // 1
	ClientEncryptedServerNot                            // 2
	ClientMaxMessageLengthTooBig                        // 3
)

type Encryption byte

const (
	Plain Encryption = iota
	Encrypted
)

// ServerConfig - used to pass configuration overrides to ServerStart()
type ServerConfig struct {
	SocketBasePath    string
	Timeout           time.Duration
	MaxMsgSize        int
	Encryption        bool
	UnmaskPermissions bool
}

// ClientConfig - used to pass configuration overrides to ClientStart()
type ClientConfig struct {
	SocketBasePath string
	Timeout        time.Duration
	RetryTimer     time.Duration
	MaxMsgSize     int
	Encryption     bool
}

// Encryption - encryption settings
type encryption struct {
	keyExchange string
	encryption  string
	cipher      *cipher.AEAD
}
