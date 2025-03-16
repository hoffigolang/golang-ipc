package ipc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	log "github.com/igadmg/golang-ipc/ipclogging"
)

// on connection wish of a client, the server initiates the handshake.
// (handshake between client and server is done purely over the connection, no go channels involved)
// handshake message 1: byte 0 = ipcVersion, byte 1 = exchange messages encrypted: =1, not-encrypted =0
// handshake message 2 (optional): exchange encryption keys (and encrypt anything that goes over the wire afterward)
// handshake message 3: byte0-4 = server's possible MaxMsgSize size as uint32 in big endian
func (s *Server) serverHandshake() error {
	err := s.serverSendAndReceiveHandshake1()
	if err != nil {
		return err
	}

	if s.conf.Encryption {
		err = s.serverExchangeEncryptionKeysAndCreateCipher()
		if err != nil {
			return err
		}
	}

	err = s.serverSendMaxMsgSizeConstraint()
	if err != nil {
		return err
	}

	log.Debugln("server handshake: successful.")
	return nil
}

func (s *Server) serverSendAndReceiveHandshake1() error {
	buff := make([]byte, 2)
	buff[0] = byte(ipcVersion)

	if s.conf.Encryption {
		buff[1] = byte(Encrypted)
	} else {
		buff[1] = byte(Plain)
	}

	_, err := s.conn.Write(buff)
	if err != nil {
		return errors.New("server handshake1: unable to send handshake1")
	} else {
		log.Debugf("server handshake1: sent handshake to client:  (version/encryption): (%d/%d)", ipcVersion, buff[1])
	}

	recv := make([]byte, 1)
	_, err = s.conn.Read(recv)
	if err != nil {
		return errors.New("server handshake1: failed to received handshake1 reply")
	} else {
		if recv[0] == 0 {
			log.Debugln("server handshake1: received handshake1 from client: ok")
		} else {
			log.Debugf("server handshake1: received handshake1 from client error: %d", recv[0])
		}
	}

	switch result := recv[0]; result {
	case 0:
		return nil
	case 1:
		return errors.New("server handshake1: client has a different version number")
	case 2:
		return errors.New("server handshake1: client is enforcing encryption")
	case 3:
		return errors.New("server handshake1: failed to get handshake reply")
	default:
		return errors.New("server handshake1: other error - handshake failed")
	}
}

func (s *Server) serverExchangeEncryptionKeysAndCreateCipher() error {
	ownPrivateKey, clientsPublicKey, err := s.serverKeyExchange()
	if err != nil {
		return err
	}

	aeadCipher, err := createCipherViaEcdsaSharedSecret(ownPrivateKey, clientsPublicKey)
	if err != nil {
		return err
	}

	s.cipher = aeadCipher
	return nil
}

func (s *Server) serverSendMaxMsgSizeConstraint() error {
	toSend := make([]byte, 4)
	buff := make([]byte, 4)
	binary.BigEndian.PutUint32(buff, uint32(s.conf.MaxMsgSize))

	if s.conf.Encryption {
		encryptedMsg, err := encrypt(s.cipher, buff)
		if err != nil {
			return err
		}

		binary.BigEndian.PutUint32(toSend, uint32(len(encryptedMsg)))
		toSend = append(toSend, encryptedMsg...)
	} else {
		binary.BigEndian.PutUint32(toSend, uint32(len(buff)))
		toSend = append(toSend, buff...)
	}

	_, err := s.conn.Write(toSend)
	if err != nil {
		return errors.New("server handshake2: unable to send MaxMsgSize constraint")
	} else {
		log.Debugln("server handshake2: sent server's MaxMsgSize constraint")
	}

	reply := make([]byte, 1)
	_, err = s.conn.Read(reply)
	if err != nil {
		return errors.New("server handshake2: did not receive MaxMsgSize constraint reply")
	} else {
		log.Debugln("server handshake2: received client's MaxMsgSize reply")
	}

	return nil
}

// after the server initiated the handshake
// (handshake between client and server is done purely over the connection, no go channels involved)
// handshake message 1: byte 0 = ipcVersion, byte 1 = exchange messages encrypted: =1, not-encrypted =0
// handshake message 2 (optional): exchange encryption keys (and encrypt anything that goes over the wire afterward)
// handshake message 3: byte0-4 = server's possible MaxMsgSize size as uint32 in big endian
func (c *Client) clientDoPassiveHandshake() error {
	err := c.clientReceiveAndSendHandshake1()
	if err != nil {
		return err
	}

	if c.conf.Encryption {
		err := c.clientDoPassiveExchangeEncryptionKeysAndCreateCipher()
		if err != nil {
			return err
		}
	}

	err = c.clientReceiveMaxMsgSizeConstraint()
	if err != nil {
		return err
	}

	log.Debugln("client handshake: successful.")
	return nil
}

func (c *Client) clientReceiveAndSendHandshake1() error {
	bytesFromServer := make([]byte, 2)
	_, err := c.conn.Read(bytesFromServer)
	if err != nil {
		return errors.New("client failed to received handshake message")
	} else {
		log.Debugf("client handshake1: received (version/encryption): (%d/%d)", bytesFromServer[0], bytesFromServer[1])
	}

	if bytesFromServer[0] != ipcVersion {
		c.handshakeSendReply(IpcVersionMismatch)
		return errors.New("client handshake: server has a different ipcVersion number")
	}

	if bytesFromServer[1] == byte(Plain) && c.conf.Encryption {
		c.handshakeSendReply(ClientEncryptedServerNot)
		return errors.New("client handshake: server communicates unencrypted/plain, client wants encrypted communication")
	}

	if bytesFromServer[1] == byte(Plain) {
		c.conf.Encryption = false
	} else {
		c.conf.Encryption = true
	}

	log.Debugln("client handshake1: sending handshake1 ok back to to server")
	c.handshakeSendReply(HandshakeOk) // 0 is ok
	return nil
}

func (c *Client) clientDoPassiveExchangeEncryptionKeysAndCreateCipher() error {
	ownPrivateKey, serversPublicKey, err := c.clientKeyExchange()
	if err != nil {
		return err
	}

	aeadCipher, err := createCipherViaEcdsaSharedSecret(ownPrivateKey, serversPublicKey)
	if err != nil {
		return err
	}

	c.cipher = aeadCipher
	return nil
}

func (c *Client) clientReceiveMaxMsgSizeConstraint() error {
	bytesFromServer := make([]byte, 4)
	_, err := c.conn.Read(bytesFromServer)
	if err != nil {
		return errors.New("client handshake2: failed to receive max message length 1")
	}

	var msgLen uint32
	binary.Read(bytes.NewReader(bytesFromServer), binary.BigEndian, &msgLen) // message length

	bytesFromServer = make([]byte, int(msgLen))
	_, err = c.conn.Read(bytesFromServer)
	if err != nil {
		return errors.New("client handshake2: failed to receive max message length 2")
	}
	var buff2 []byte
	if c.conf.Encryption {
		buff2, err = decrypt(c.cipher, bytesFromServer)
		if err != nil {
			return errors.New("client handshake2: failed to receive max message length 3")
		}
	} else {
		buff2 = bytesFromServer
	}

	var maxMsgSize uint32
	binary.Read(bytes.NewReader(buff2), binary.BigEndian, &maxMsgSize) // message length

	if c.conf.MaxMsgSize > 0 {
		maxMsgLenOfServer := bytesToInt(bytesFromServer[:2])
		if maxMsgLenOfServer > 0 && maxMsgLenOfServer < c.conf.MaxMsgSize {
			c.handshakeSendReply(ClientMaxMessageLengthTooBig)
			return errors.New(fmt.Sprintf("client handshake2: server only supports message length up to %d", maxMsgLenOfServer))
		}
	} else {
		c.conf.MaxMsgSize = int(maxMsgSize)
	}

	log.Debugln("client handshake2: sending handshake2 ok with server's maxMsgSize constraint")
	c.handshakeSendReply(HandshakeOk)
	return nil
}

func (c *Client) handshakeSendReply(result HandshakeResult) {
	buff := make([]byte, 1)
	buff[0] = byte(result)

	c.conn.Write(buff)
}
