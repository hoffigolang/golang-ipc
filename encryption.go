package ipc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"io"
	"net"
)

var ellipticCurve = elliptic.P384() // sharedSecretByteLength = 48 marshalledPublicKeyByteLength = 97 + 1
//var ellipticCurve = elliptic.P256() // sharedSecretByteLength = 32 marshalledPublicKeyByteLength = 65 + 1

// serverKeyExchange - get other side's public key
func (s *Server) serverKeyExchange() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	priv, pub, err := generateECDHKeyPair()
	if err != nil {
		return nil, nil, err
	}

	// send servers public key
	err = sendPublicKey("server handshake:", s.conn, pub)
	if err != nil {
		return nil, nil, err
	}

	// received clients public key
	otherSidesPublicKey, err := receivePublicKey("server handshake:", s.conn)
	if err != nil {
		return nil, nil, err
	}

	if priv.Params().Name != otherSidesPublicKey.Params().Name {
		return nil, nil, errors.New(fmt.Sprintf("server: own private key with ecdsa '%s' other side's with '%s' don't match",
			priv.Params().Name, otherSidesPublicKey.Params().Name))
	}

	return priv, otherSidesPublicKey, nil
}

func (c *Client) clientKeyExchange() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	priv, pub, err := generateECDHKeyPair()
	if err != nil {
		return nil, nil, err
	}

	// received servers public key
	otherSidesPublicKey, err := receivePublicKey("client handshake:", c.conn)
	if err != nil {
		return nil, nil, err
	}

	// send clients public key
	err = sendPublicKey("client handshake:", c.conn, pub)
	if err != nil {
		return nil, nil, err
	}

	if priv.Params().Name != otherSidesPublicKey.Params().Name {
		return nil, nil, errors.New(fmt.Sprintf("client: own private key with ecdsa '%s' other side's with '%s' don't match",
			priv.Params().Name, otherSidesPublicKey.Params().Name))
	}

	return priv, otherSidesPublicKey, nil
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
		log.Debugln(who + " sent public key to other side")
	}

	return nil
}

func receivePublicKey(who string, conn net.Conn) (*ecdsa.PublicKey, error) {
	var buff []byte
	switch ellipticCurve.Params().Name {
	case "P-384":
		buff = make([]byte, 65+1)
	case "P-256":
		buff = make([]byte, 97+1)
	}
	n, err := conn.Read(buff)
	if err != nil {
		return nil, errors.New(who + " didn't received public key")
	} else {
		log.Debugln(who + " received public key")
	}

	recvdPub := bytesToPublicKey(buff[:n])
	return recvdPub, nil
}

// ============================================================================
// ============================================================================
// ============================================================================

// generateECDHKeyPair generates an ECDH key pair using P-384.
func generateECDHKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privateKey, err := ecdsa.GenerateKey(ellipticCurve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

func publicKeyToBytes(pubkey *ecdsa.PublicKey) []byte {
	return elliptic.MarshalCompressed(ellipticCurve, pubkey.X, pubkey.Y)
}
func bytesToPublicKey(b []byte) *ecdsa.PublicKey {
	if len(b) == 0 {
		return nil
	}
	x, y := elliptic.UnmarshalCompressed(ellipticCurve, b)
	return &ecdsa.PublicKey{Curve: ellipticCurve, X: x, Y: y}
}

func createCipherViaEcdsaSharedSecret(ownPrivKey *ecdsa.PrivateKey, otherSidePubKey *ecdsa.PublicKey) (*cipher.AEAD, error) {
	sharedSecretHash, err := sharedSecretSha256(ownPrivKey, otherSidePubKey)
	if err != nil {
		return nil, err
	}
	aeadCipher, err := createCipher(sharedSecretHash)
	if err != nil {
		return nil, err
	}
	return aeadCipher, nil
}

func sharedSecretSha256(ownPrivKey *ecdsa.PrivateKey, otherSidePubKey *ecdsa.PublicKey) ([sha256.Size]byte, error) {
	if ownPrivKey.Curve != otherSidePubKey.Curve {
		return [sha256.Size]byte{}, fmt.Errorf("used ecdsa curves do not match")
	}
	sharedSecret, _ := otherSidePubKey.Curve.ScalarMult(otherSidePubKey.X, otherSidePubKey.Y, ownPrivKey.D.Bytes())
	hashedSharedSecret := sha256.Sum256(sharedSecret.Bytes())
	return hashedSharedSecret, nil
}

// createCipher creates an Authenticated encryption with associated data (AEAD) cipher
// using the generated and sha256 hashed sharedSecretSha256 calculated from "other-side"'s public ecdsa-key and own private ecdsa-key
func createCipher(sharedSecret [sha256.Size]byte) (*cipher.AEAD, error) {
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
