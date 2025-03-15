package ipc

func NewMessage(msgType MsgType, data []byte) *Message {
	return &Message{
		Err:     nil,
		IpcType: None,
		MsgType: msgType,
		Data:    data,
	}
}
func NewStringMessage(data string) *Message {
	return &Message{
		Err:     nil,
		IpcType: None,
		MsgType: String,
		Status:  None,
		Data:    []byte(data),
	}
}

func NewCIpcLocalStatusMessage(status ClientStatus) *Message {
	return NewIpcLocalStatusMessage(Status(status))
}
func NewSIpcLocalStatusMessage(status ServerStatus) *Message {
	return NewIpcLocalStatusMessage(Status(status))
}
func NewIpcLocalStatusMessage(status Status) *Message {
	return &Message{
		Err:     nil,
		IpcType: IpcLocalMsg,
		MsgType: String, // any
		Status:  status,
		Data:    nil,
	}
}
func NewCIpcRemoteStatusMessage(status ClientStatus) *Message {
	return NewIpcRemoteStatusMessage(Status(status))
}
func NewSIpcRemoteMessage(status ServerStatus) *Message {
	return NewIpcRemoteStatusMessage(Status(status))
}
func NewIpcRemoteStatusMessage(status Status) *Message {
	return &Message{
		Err:     nil,
		IpcType: IpcRemoteMsg,
		MsgType: String, // any
		Status:  status,
		Data:    nil,
	}
}
func NewIpcMessage(ipcMsgType IpcMsgType, data []byte) *Message {
	return &Message{
		Err:     nil,
		IpcType: ipcMsgType,
		MsgType: String, // any
		Status:  None,
		Data:    data,
	}
}
func NewIpcErrorMessage(err error) *Message {
	return &Message{
		Err:     err,
		IpcType: OtherError,
		MsgType: Error,
		Status:  None,
		Data:    nil,
	}
}
func NewIpcConnectionErrorMessage(err error) *Message {
	return &Message{
		Err:     err,
		IpcType: ConnectionError,
		MsgType: Error,
		Status:  None,
		Data:    nil,
	}
}
