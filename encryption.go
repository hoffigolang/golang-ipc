package ipc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"io"
	"net"
)

var ellipticCurve = elliptic.P384()
var sharedSecretByteLength = 48
var marshalledPublicKeyByteLength = 97

func (s *Server) serverKeyExchangeAndCreateSharedSecret() ([32]byte, error) {
	var shared [32]byte // sha256 of sharedSecret

	priv, pub, err := generateECDHKeyPair()
	if err != nil {
		return shared, err
	}

	// send servers public key
	err = sendPublicKey("server handshake:", s.conn, pub)
	if err != nil {
		return shared, err
	}

	// received clients public key
	pubRecvd, err := receivePublicKey("server handshake:", s.conn)
	if err != nil {
		return shared, err
	}

	b, _ := pubRecvd.Curve.ScalarMult(pubRecvd.X, pubRecvd.Y, priv.D.Bytes())
	shared = sha256.Sum256(b.Bytes())

	return shared, nil
}

func (c *Client) keyExchange() ([32]byte, error) {
	var shared [32]byte

	priv, pub, err := generateECDHKeyPair()
	if err != nil {
		return shared, err
	}

	// received servers public key
	pubRecvd, err := receivePublicKey("client handshake:", c.conn)
	if err != nil {
		return shared, err
	}

	// send clients public key
	err = sendPublicKey("client handshake:", c.conn, pub)
	if err != nil {
		return shared, err
	}

	b, _ := pubRecvd.Curve.ScalarMult(pubRecvd.X, pubRecvd.Y, priv.D.Bytes())
	shared = sha256.Sum256(b.Bytes())

	return shared, nil
}

func generateECDHKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privateKey, err := ecdsa.GenerateKey(ellipticCurve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

func sendPublicKey(who string, conn net.Conn, pub *ecdsa.PublicKey) error {
	pubSend := publicKeyToBytes(pub)
	if pubSend == nil {
		return errors.New(who + " public key cannot be converted to bytes")
	}

	_, err := conn.Write(pubSend)
	if err != nil {
		return errors.New(who + " could not sent public key")
	} else {
		log.Debugln(who + " sent public key to client")
	}

	return nil
}

func receivePublicKey(who string, conn net.Conn) (*ecdsa.PublicKey, error) {
	buff := make([]byte, 98)
	i, err := conn.Read(buff)
	if err != nil {
		return nil, errors.New(who + " didn't received public key")
	} else {
		log.Debugln(who + " received public key")
	}

	if i != 97 {
		return nil, errors.New(who + " public key received isn't valid length")
	}

	recvdPub := bytesToPublicKey(buff[:i])
	if !recvdPub.IsOnCurve(recvdPub.X, recvdPub.Y) {
		return nil, errors.New(who + " didn't received valid public key")
	}

	return recvdPub, nil
}

func publicKeyToBytes(pub *ecdsa.PublicKey) []byte {
	return elliptic.Marshal(ellipticCurve, pub.X, pub.Y) // TODO
}

func bytesToPublicKey(recvdPub []byte) *ecdsa.PublicKey {
	if len(recvdPub) == 0 {
		return nil
	}
	x, y := elliptic.Unmarshal(elliptic.P384(), recvdPub) // TODO
	return &ecdsa.PublicKey{Curve: elliptic.P384(), X: x, Y: y}
}

func createCipher(shared [32]byte) (*cipher.AEAD, error) {
	b, err := aes.NewCipher(shared[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}

	return &gcm, nil
}

func encrypt(g cipher.AEAD, data []byte) ([]byte, error) {
	nonce := make([]byte, g.NonceSize())
	_, err := io.ReadFull(rand.Reader, nonce)

	return g.Seal(nonce, nonce, data, nil), err
}

func decrypt(g cipher.AEAD, recdData []byte) ([]byte, error) {
	nonceSize := g.NonceSize()
	if len(recdData) < nonceSize {
		return nil, errors.New("not enough data to decrypt")
	}

	nonce, recdData := recdData[:nonceSize], recdData[nonceSize:]
	plain, err := g.Open(nil, nonce, recdData, nil)
	if err != nil {
		return nil, err
	}

	return plain, nil
}
