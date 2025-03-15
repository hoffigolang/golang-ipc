package ipc

import "time"

const ipcVersion = 2 // ipc ipcVersion for assuring message compatibility
const FinalMessage = "°§°finalMessage°§°"
const IntermediateActionMessage = "°§°aaaaandAction°§°"
const InitialMessage = "°§°initialMessage°§°"
const (
	minMsgSize        = 1024
	defaultMaxMsgSize = 3145728 // 3Mb  - Maximum bytes allowed for each message
	defaultRetryTimer = time.Duration(200 * time.Millisecond)
)

var (
	DefaultServerConfig = ServerConfig{
		SocketBasePath:    defaultSocketBasePath,
		Timeout:           0,
		MaxMsgSize:        defaultMaxMsgSize,
		Encryption:        false,
		UnmaskPermissions: true,
	}

	DefaultClientConfig = ClientConfig{
		SocketBasePath: defaultSocketBasePath,
		Timeout:        0,
		RetryTimer:     defaultRetryTimer,
		MaxMsgSize:     defaultMaxMsgSize,
		Encryption:     false,
	}
)
