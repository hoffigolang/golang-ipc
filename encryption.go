package ipc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"github.com/hoffigolang/golang-ipc/encryption"
	log "github.com/hoffigolang/golang-ipc/ipclogging"
	"golang.org/x/crypto/curve25519"
	"io"
	"net"
)

var ellipticCurve = elliptic.P384() // sharedSecretByteLength = 48 marshalledPublicKeyByteLength = 97 + 1
//var ellipticCurve = elliptic.P256() // sharedSecretByteLength = 32 marshalledPublicKeyByteLength = 65 + 1

// serverKeyExchange - get other side's public key
func (s *Server) serverKeyExchange() (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	priv, err := encryption.NewX25519KeyPair()
	if err != nil {
		return nil, nil, err
	}
	pub := priv.PublicKey()

	// send servers public key
	err = sendPublicKey("server handshake:", s.conn, pub)
	if err != nil {
		return nil, nil, err
	}

	// received clients public key
	peerPubKey, err := receivePublicKey("server handshake:", s.conn)
	if err != nil {
		return nil, nil, err
	}

	return priv, peerPubKey, nil
}

func (c *Client) clientKeyExchange() (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	priv, err := encryption.NewX25519KeyPair()
	if err != nil {
		return nil, nil, err
	}
	pub := priv.PublicKey()

	// received servers public key
	peerPubKey, err := receivePublicKey("client handshake:", c.conn)
	if err != nil {
		return nil, nil, err
	}

	// send clients public key
	err = sendPublicKey("client handshake:", c.conn, pub)
	if err != nil {
		return nil, nil, err
	}

	return priv, peerPubKey, nil
}

func sendPublicKey(who string, conn net.Conn, pub *ecdh.PublicKey) error {
	pubSend := pub.Bytes()
	if pubSend == nil || len(pubSend) == 0 {
		return errors.New(who + " public key cannot be converted to bytes")
	}

	_, err := conn.Write(pubSend)
	if err != nil {
		return errors.New(who + " could not sent public key")
	} else {
		log.Debugln(who + " sent public key to other side")
	}

	return nil
}

func receivePublicKey(who string, conn net.Conn) (*ecdh.PublicKey, error) {
	buff := make([]byte, 32)
	_, err := conn.Read(buff)
	if err != nil {
		return nil, errors.New(who + " didn't received public key")
	} else {
		log.Debugln(who + " received public key")
	}

	recvdPub, err := ecdh.X25519().NewPublicKey(buff)
	if err != nil {
		return nil, errors.New(who + " " + err.Error())
	}
	return recvdPub, nil
}

// ============================================================================
// ============================================================================
// ============================================================================

func sharedSecretX25519(privKeyBytes []byte, peerPubKeyBytes []byte) ([32]byte, error) {
	if len(peerPubKeyBytes) != 32 {
		return [32]byte{}, errors.New("peer's public key is not 32 bytes long")
	}
	sharedSecret, err := curve25519.X25519(privKeyBytes, peerPubKeyBytes)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(sharedSecret), nil
}

// createCipher creates an Authenticated encryption with associated data (AEAD) cipher
// using the generated and sha256 hashed sharedSecretSha256 calculated from "other-side"'s public ecdsa-key and own private ecdsa-key
func createGcmCipherFromX25519SharedKey(sharedSecret [sha256.Size]byte) (*cipher.AEAD, error) {
	b, err := aes.NewCipher(sharedSecret[:])
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}

	return &aesGCM, nil
}

func encrypt(cipher *cipher.AEAD, data []byte) ([]byte, error) {
	nonce := make([]byte, (*cipher).NonceSize())
	_, err := io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	return (*cipher).Seal(nonce, nonce, data, nil), err
}

func decrypt(cipher *cipher.AEAD, encodedData []byte) ([]byte, error) {
	nonceSize := (*cipher).NonceSize()
	if len(encodedData) < nonceSize {
		return nil, errors.New("not enough data to decrypt")
	}

	nonce, encodedData := encodedData[:nonceSize], encodedData[nonceSize:]
	plain, err := (*cipher).Open(nil, nonce, encodedData, nil)
	if err != nil {
		return nil, err
	}

	return plain, nil
}
