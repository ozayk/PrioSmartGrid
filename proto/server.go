package proto

import (
	"errors"
	"log"
	"math/big"
	"sync"
	"fmt"

	"../config"
	"../mpc"
)

var pargs PublishArgs

func NewServer(cfg *config.Config, idx int) *Server {
	s := new(Server)
	s.ServerIdx = idx
	s.toProcess = make(chan *UploadArgs)
	s.pending = make(map[Uuid]*RequestStatus)
	s.cfg = cfg

	ns := cfg.NumServers()

	log.Printf("[yoza] NewServer number of servers is:", ns)
	s.agg = make([]*mpc.Aggregator, ns)
	s.aggEpoch = make([]uint, ns)
	s.aggMutex = make([]sync.Mutex, ns)

	s.storedAgg = make(map[uint]*mpc.Aggregator)
	s.storedAggCount = make(map[uint]uint)

	s.ClientAgg = make(map[uint]*mpc.Aggregator)
	s.KeyToID = make(map[Uuid]uint)

	s.storedClientAgg = make(map[uint]*mpc.Aggregator)

	s.nProcessed = make([]uint, ns)
	s.nProcessedMutex = make([]sync.Mutex, ns)
	s.nProcessedCond = make([]*sync.Cond, ns)

	s.pre = make([]*mpc.CheckerPrecomp, ns)
	for i := 0; i < ns; i++ {
		s.pre[i] = mpc.NewCheckerPrecomp(s.cfg)
	}

	for i := 0; i < ns; i++ {
		s.agg[i] = mpc.NewAggregator(s.cfg)
		s.nProcessedCond[i] = sync.NewCond(&s.nProcessedMutex[i])
	}

	s.randomX = make([]*big.Int, cfg.NumServers())
	s.randomXMutex = make([]sync.Mutex, ns)

	s.pool = make([]*checkerPool, cfg.NumServers())
	for leaderIdx := 0; leaderIdx < cfg.NumServers(); leaderIdx++ {
		s.pool[leaderIdx] = NewCheckerPool(s.cfg, s.ServerIdx, leaderIdx)
	}

	return s
}

func (s *Server) isLeader() bool {
	return s.ServerIdx == 0
}

func (s *Server) NewRequest(args *NewRequestArgs, reply *NewRequestReply) error {
	// Add request to queue
	r, err := decryptRequest(s.ServerIdx, &args.RequestID, &args.Ciphertext)
	if err != nil {
		log.Print("Could not decrypt insert args")
		return err
	}

	dstServer := HashToServer(s.cfg, args.RequestID)

	s.pendingMutex.RLock()
	exists := s.pending[args.RequestID] != nil
	s.pendingMutex.RUnlock()

	if exists {
		log.Print(s.pending[args.RequestID])
		log.Print("Error: Key collision! Ignoring bogus request.")
		return nil
	}

	status := new(RequestStatus)
	status.check = s.pool[dstServer].get()
	status.check.SetReq(r)
	if status.check == nil {
		log.Printf("Warning: ignoring invalid client request (%v)", args.RequestID)
		return nil
	}


	status.flag = NotStarted

	s.pendingMutex.Lock()
	s.KeyToID[args.RequestID] = args.ClientID
	s.pending[args.RequestID] = status
	s.pendingMutex.Unlock()

	return nil
}

func (s *Server) EvalCircuit(args *EvalCircuitArgs, reply *mpc.CorShare) error {
	leader := HashToServer(s.cfg, args.RequestID)

	s.pendingMutex.RLock()
	status, okay := s.pending[args.RequestID]
	s.pendingMutex.RUnlock()
	if !okay {
		return errors.New("Could not find specified request")
	}

	if status.flag != NotStarted {
		return errors.New("Request already processed")
	}

	s.randomXMutex[leader].Lock()
	if s.randomX[leader] != nil {
		s.pre[leader].SetCheckerPrecomp(s.randomX[leader])
	}
	s.randomX[leader] = nil
	s.randomXMutex[leader].Unlock()

	status.flag = Layer1
	status.check.CorShare(reply, s.pre[leader])

	log.Print("Done evaluating ", args.RequestID)
	return nil
}

func (s *Server) FinalCircuit(args *FinalCircuitArgs, reply *mpc.OutShare) error {

	s.pendingMutex.RLock()
	status, okay := s.pending[args.RequestID]
	s.pendingMutex.RUnlock()
	if !okay {
		return errors.New("Could not find specified request")
	}

	if status.flag != Layer1 {
		return errors.New("Request already processed")
	}
	status.flag = Finished

	status.check.OutShare(reply, args.Cor, args.Key)

	return nil
}

func (s *Server) Accept(args *AcceptArgs, reply *AcceptReply) error {
	s.pendingMutex.RLock()
	status, okay := s.pending[args.RequestID]
	s.pendingMutex.RUnlock()
	if !okay {
		return errors.New("Could not find specified request")
	}

	if status.flag != Finished {
		return errors.New("Request not yet processed")
	}

	s.pendingMutex.Lock()
	delete(s.pending, args.RequestID)
	s.pendingMutex.Unlock()

	l := HashToServer(s.cfg, args.RequestID)
	log.Print("[yoza] HashToServer: ", l)
	if args.Accept {
		s.aggMutex[l].Lock()
		s.agg[l].Update(status.check)

		_, okay := s.ClientAgg[args.ClientID]
		if !okay {
			s.ClientAgg[args.ClientID] = mpc.NewAggregator(s.cfg)
			s.ClientAgg[args.ClientID].Names[0] = fmt.Sprintf("Client-%v", args.ClientID)
			log.Printf("[yoza] Creating new ClientAgg for client: %v", args.ClientID)
		}
		s.ClientAgg[args.ClientID].Update(status.check)
		log.Printf("[yoza] Printing ClientAgg")
		log.Print(s.ClientAgg)

		s.aggMutex[l].Unlock()

		s.nProcessedCond[l].Signal()
		s.nProcessedMutex[l].Lock()
		s.nProcessed[l]++
		fmt.Println("[yoza]: increament nProcessed, Now :", s.nProcessed[l])
		s.nProcessedMutex[l].Unlock()
		s.nProcessedCond[l].Signal()
	}

	log.Printf("Done!")
	s.pool[l].put(status.check)

	return nil
}

func (s *Server) Aggregate(args *AggregateArgs, reply *AggregateReply) error {
	if args.Server >= uint(s.cfg.NumServers()) {
		return errors.New("Bogus server id")
	}

	// TODO: Check that number of aggregated values is large!
	// In other words, we want a large anonymity set!
	s.nProcessedMutex[args.Server].Lock()
	for s.nProcessed[args.Server] < args.Serial {
		s.nProcessedCond[args.Server].Wait()
	}

	s.aggMutex[args.Server].Lock()

	log.Printf("[yoza] Aggregate Printing s.agg for %v", args.Server)
	log.Print(s.agg)

	log.Printf("[yoza] Aggregate Printing s.ClientAgg for %v", args.Server)
	log.Print(s.ClientAgg)


	//var pargs PublishArgs
	pargs.Server = uint(s.ServerIdx)
	pargs.Epoch = s.aggEpoch[args.Server]
	//pargs.Agg = s.agg[args.Server].Copy()

	if pargs.Agg == nil {
		pargs.Agg = mpc.NewAggregator(s.cfg)
		log.Printf("[yoza] Creating new Aggregrator!")
	}

	pargs.Agg.Combine(s.agg[args.Server])

	if pargs.ClientAgg == nil {
		pargs.ClientAgg = make(map[uint]*mpc.Aggregator)
		log.Printf("[yoza] called make for pargs.ClientAgg")
	}

/**
	for k,v := range s.ClientAgg {

		_, okay := pargs.ClientAgg[k]
		if !okay {
			pargs.ClientAgg[k] = mpc.NewAggregator(s.cfg)
			pargs.ClientAgg[k] = v
			log.Printf("[yoza] Creating new ClientAgg for final Aggregator, ClientID is: %v", k)
		} else {
			pargs.ClientAgg[k] = v
			log.Printf("[yoza] Copying ClientAgg for final Aggregator, ClientID is: %v", k)
		}

	}
**/
/**
	for k := range s.ClientAgg {

		_, okay := pargs.ClientAgg[k]
		if !okay {
			pargs.ClientAgg[k] = mpc.NewAggregator(s.cfg)
			pargs.ClientAgg[k].Names[0] = fmt.Sprintf("Client-%v", k)
			log.Printf("[yoza] Creating new ClientAgg for final Aggregator, ClientID is: %v", k)
		}

		log.Printf("[yoza] printing s.ClientAgg while populating pargs for key:%v", k)
		log.Print(s.ClientAgg[k])

		pargs.ClientAgg[k].Combine(s.ClientAgg[k])

		log.Printf("[yoza] printing pargs.ClientAgg while populating pargs for key:%v", k)
		log.Print(pargs.ClientAgg[k])


	}
**/
	s.aggEpoch[args.Server]++


	// Wait until we have received an Accept() request
	// from all other servers.
	ready := true
	log.Printf("Epochs: %v", s.aggEpoch)
	for i := 0; i < s.cfg.NumServers(); i++ {
		if s.aggEpoch[i] < s.aggEpoch[args.Server] {
			ready = false
			log.Printf("[yoza] Aggregate turning ready to FALSE")
		}
	}

	if ready {
		var pubReply PublishReply
		log.Printf("[yoza] Aggregate calling Publish....")
		log.Print(pargs)
		log.Printf("\n\n")
		log.Print(pargs.Agg)
		log.Printf("\n\n")

		for k,v := range s.ClientAgg {

			_, okay := pargs.ClientAgg[k]
			if !okay {
				pargs.ClientAgg[k] = mpc.NewAggregator(s.cfg)
				pargs.ClientAgg[k] = v
				pargs.ClientAgg[k].Names[0] = fmt.Sprintf("Client-%v", k)
				log.Printf("[yoza] Creating new ClientAgg for final Aggregator, ClientID is: %v", k)
			} else {
				pargs.ClientAgg[k] = v
				log.Printf("[yoza] Copying ClientAgg for final Aggregator, ClientID is: %v", k)
			}

		}

		log.Print(pargs.ClientAgg)
		log.Printf("\n\n")
		err := s.LeaderClient.Call("Server.Publish", &pargs, &pubReply)
		if err != nil {
			log.Fatalf("Publish error: %v", err)
		}
	}

	// To be conservative, hold the lock until the publish operation
	// completes.
	s.agg[args.Server].Reset()
	s.aggMutex[args.Server].Unlock()

	s.nProcessedMutex[args.Server].Unlock()

	return nil
}

func (s *Server) Publish(args *PublishArgs, reply *PublishReply) error {
	if !s.isLeader() {
		log.Printf("[yoza] Publish: NOT a leader!")
		return errors.New("Am not leader!")
	}

	log.Printf("[yoza] Publish before lock...")
	s.storedAggMutex.Lock()

	log.Printf("[yoza] Printing whole pargs")
	log.Print(args)

	log.Printf("[yoza] Publish during lock...")
	_, okay := s.storedAgg[args.Epoch]
	if !okay {
		s.storedAgg[args.Epoch] = mpc.NewAggregator(s.cfg)
		s.storedAggCount[args.Epoch] = 0
		log.Printf("[yoza] Publish not OKAY")
	}

	s.storedAgg[args.Epoch].Combine(args.Agg)
	s.storedAggCount[args.Epoch]++

	for k := range args.ClientAgg {
		_, ok := s.storedClientAgg[k]
		if !ok {
			s.storedClientAgg[k] = mpc.NewAggregator(s.cfg)
			s.storedClientAgg[k].Names[0] = fmt.Sprintf("Client-%v", k)
			log.Printf("[yoza] Creating new storedClientAgg for final Aggregator, ClientID is: %v", k)
		}

		s.storedClientAgg[k].Combine(args.ClientAgg[k])
		log.Printf("[yoza] Combining storedClientAgg for final Aggregator, ClientID is: %v", k)
		log.Print(s.storedClientAgg[k])
	}

	if s.storedAggCount[args.Epoch] == uint(s.cfg.NumServers()) {
		log.Print(s.storedAgg[args.Epoch])
		log.Printf("[yoza] Publish storedAgg for Epoch(%d) is: %v", args.Epoch, s.storedAgg[args.Epoch])

		log.Printf("\n[yoza] Publish final ClientAgg\n")
		for key,value := range s.storedClientAgg {
			log.Printf("[yoza] Aggregate for client-%v :\n", key)
			log.Print(value)
		}

		delete(s.storedAgg, args.Epoch)
		delete(s.storedAggCount, args.Epoch)
		for k := range s.storedClientAgg {
			delete(s.storedClientAgg, k)
		}

	}

	s.storedAggMutex.Unlock()

	return nil
}

func (s *Server) ChangePolyPoint(args *ChangePolyPointArgs, reply *int) error {
	if args.Server >= s.cfg.NumServers() {
		return errors.New("Bogus server ID")
	}

	s.randomXMutex[args.Server].Lock()
	s.randomX[args.Server] = args.RandomX
	s.randomXMutex[args.Server].Unlock()
	return nil
}
