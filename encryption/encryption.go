package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"golang.org/x/crypto/curve25519"
	"io"
)

var ellipticCurve = elliptic.P384() // sharedSecretByteLength = 48 marshalledPublicKeyByteLength = 97 + 1
//var ellipticCurve = elliptic.P256() // sharedSecretByteLength = 32 marshalledPublicKeyByteLength = 65 + 1

func NewX25519KeyPair() (*ecdh.PrivateKey, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return priv, nil
}

func SharedSecretX25519(privKeyBytes []byte, peerPubKeyBytes []byte) ([32]byte, error) {
	if len(peerPubKeyBytes) != 32 {
		return [32]byte{}, errors.New("peer's public key is not 32 bytes long")
	}
	sharedSecret, err := curve25519.X25519(privKeyBytes, peerPubKeyBytes)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(sharedSecret), nil
}

// CreateGcmCipherFromX25519SharedKey creates an Authenticated encryption with associated data (AEAD) cipher
// using the generated and sha256 hashed sharedSecretSha256 calculated from "other-side"'s public ecdsa-key and own private ecdsa-key
func CreateGcmCipherFromX25519SharedKey(sharedSecret [sha256.Size]byte) (*cipher.AEAD, error) {
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

func Encrypt(cipher *cipher.AEAD, data []byte) ([]byte, error) {
	nonce := make([]byte, (*cipher).NonceSize())
	_, err := io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	return (*cipher).Seal(nonce, nonce, data, nil), err
}

func Decrypt(cipher *cipher.AEAD, encodedData []byte) ([]byte, error) {
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
