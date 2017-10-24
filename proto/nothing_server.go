package proto

import (
	"sync"
	"time"
	"log"
)

// A server that does nothing, just for performance
// comparison purposes.
type NothingServer struct {
	done      uint
	doneMutex sync.Mutex
	counter   stats
}

func NewNothingServer() *NothingServer {
	s := new(NothingServer)
	go s.counter.PrintEvery(10 * time.Second)
	return s
}

func (s *NothingServer) Upload(args *UploadArgs, reply *UploadReply) error {
	var uuid Uuid
	uuid = args.PublicKey
	_, err := decryptRequest(0, &uuid, &args.Ciphertexts[0])
	if err != nil {
		panic("Error")
	}

	log.Printf("[yoza] NothingServer Upload before lock")
	s.doneMutex.Lock()
	s.done += 1
	log.Printf("[yoza] NothingServer Upload during lock, Value is: %v", s.done)
	s.counter.Update(s.done)
	s.doneMutex.Unlock()
	log.Printf("[yoza] NothingServer Upload after lock")
	return nil
}
