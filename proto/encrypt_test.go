package proto

import (
	"crypto/rand"
	"testing"

	"../../../go/src/golang.org/x/crypto/nacl/box"

	"../config"
	"../mpc"
)

func TestEncrypt(t *testing.T) {
	reqs := mpc.RandomRequest(config.Default, 0)

	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fail()
	}

	args, err := encryptRequest(pub, priv, 0, reqs[0])
	if err != nil {
		t.Fail()
	}

	uuid := Uuid(*pub)
	_, err = decryptRequest(0, &uuid, &args)
	if err != nil {
		t.Fail()
	}
}
