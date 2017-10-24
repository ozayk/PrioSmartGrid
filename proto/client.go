package proto

import (
	"crypto/rand"
	"log"
	"../utils"

	"../../../go/src/golang.org/x/crypto/nacl/box"

	"../config"
	"../mpc"
)

// Generate a request that targets a particular leader node.
// If the leader ID is < 0, then a random server is the leader.
func GenUploadArgs(cfg *config.Config, leaderIdx int, reqs []*mpc.ClientRequest) (*UploadArgs, error) {
	n := cfg.NumServers()

	out := new(UploadArgs)
	var err error
	var pub, priv *[32]byte

	for {
		pub, priv, err = box.GenerateKey(rand.Reader)
		log.Printf("[yoza] GenerateKey called")
		if err != nil {
			return nil, err
		}

		if HashToServer(cfg, *pub) == leaderIdx || leaderIdx < 0 {
			break
		}
	}

	leaderForReq := HashToServer(cfg, *pub)
	if reqs == nil {
		reqs = mpc.RandomRequest(cfg, leaderForReq)
	}

	out.PublicKey = *pub
	out.ClientID = uint(utils.C_ID)
	out.Ciphertexts = make([]ServerCiphertext, n)
	for s := 0; s < n; s++ {
		//log.Printf("Fields[%v] = %v", s, len(reqs[s].Fields))
		//for j := 0; j < len(reqs[s].Fields); j++ {
		//log.Printf("Fields[%v][%v] = %v", s, j, len(reqs[s].Fields[j]))
		//}

		//log.Printf("Fields[%v] = %v", s, len(reqs[s].Fields))
		//log.Printf("TripeShare[%v] = %v", s, len(reqs[s].TripleShare))
		//log.Printf("Hint[%v] = %v", s, len(reqs[s].Hint))
		out.Ciphertexts[s], err = encryptRequest(pub, priv, s, reqs[s])
		//log.Printf("Size[%v] = %v", s, len(out.Ciphertexts[s].Ciphertext))
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func GenDummyUploadArgs(cfg *config.Config, reqs []*mpc.ClientRequest) (*UploadArgs, error) {
	out := new(UploadArgs)
	var err error
	var pub, priv *[32]byte

	for {
		pub, priv, err = box.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}

		if HashToServer(cfg, *pub) == 0 {
			break
		}
	}

	if reqs == nil {
		reqs = mpc.RandomRequest(cfg, 0)
	}

	out.PublicKey = *pub
	out.Ciphertexts = make([]ServerCiphertext, 1)
	out.Ciphertexts[0], err = encryptRequest(pub, priv, 0, reqs[0])
	return out, err
}
