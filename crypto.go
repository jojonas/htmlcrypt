package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
)

type envelope struct {
	KDFIterations int
	KDFSalt       []byte
	KeyLen        int
	Ciphertext    []byte
	GCMNonce      []byte
}

func encrypt(plaintext []byte, password string, iterations int) (*envelope, error) {
	out := &envelope{
		KDFIterations: iterations,
		KDFSalt:       randomBytes(saltLen),
		KeyLen:        keyLen,
	}

	dk, err := pbkdf2.Key(sha256.New, password, out.KDFSalt, iterations, out.KeyLen)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	out.GCMNonce = randomBytes(aesgcm.NonceSize())
	out.Ciphertext = aesgcm.Seal(nil, out.GCMNonce, plaintext, nil)

	return out, nil
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	// will crash on error
	rand.Read(b)
	return b
}
