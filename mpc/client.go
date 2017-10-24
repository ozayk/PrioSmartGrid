package mpc

import (
	"log"
	"math/big"

	"../circuit"
	"../config"
	"../poly"
	"../share"
	"../triple"
	"../utils"
)

// The data struct that the client gives to each server.
type ClientRequest struct {
	Hint *share.PRGHints

	// Compressed representation of Beaver triples for the
	// batch checking and for the main MPC protocol.
	TripleShare *triple.Share
}

func sharePolynomials(ckt *circuit.Circuit, prg *share.GenPRG) {
	mulGates := ckt.MulGates()
	mod := ckt.Modulus()
	log.Printf("[yoza] Mod value is : %d", mod)

	// Little n the number of points on the polynomials.
	// The constant term is randomized, so it's (mulGates + 1).
	n := len(mulGates) + 1
	log.Printf("Mulgates: %v", n)

	// Big N is n rounded up to a power of two
	N := utils.NextPowerOfTwo(n)

	// Get the n2-th roots of unity
	pointsF := make([]*big.Int, N)
	pointsG := make([]*big.Int, N)
	zeros := make([]*big.Int, N)
	for i := 0; i < N; i++ {
		zeros[i] = utils.Zero
	}

	// Compute f(x) and g(x)
	pointsF[0] = prg.ShareRand(mod)
	pointsG[0] = prg.ShareRand(mod)

	// Send a sharing of h(0) = f(0)*g(0).
	h0 := new(big.Int)
	h0.Mul(pointsF[0], pointsG[0])
	h0.Mod(h0, mod)
	prg.Share(mod, h0)

	for i := 1; i < n; i++ {
		pointsF[i] = mulGates[i-1].ParentL.WireValue
		pointsG[i] = mulGates[i-1].ParentR.WireValue
	}

	// Zero pad the upper coefficients of f(x) and g(x)
	for i := n; i < N; i++ {
		pointsF[i] = utils.Zero
		pointsG[i] = utils.Zero
	}

	// Interpolate through the Nth roots of unity
	polyF := poly.InverseFFT(pointsF)
	polyG := poly.InverseFFT(pointsG)
	paddedF := append(polyF, zeros...)
	paddedG := append(polyG, zeros...)

	// Evaluate at all 2N-th roots of unity
	evalsF := poly.FFT(paddedF)
	evalsG := poly.FFT(paddedG)

	// We need to send to the servers the evaluations of
	//   f(r) * g(r)
	// for all 2N-th roots of unity r that are not also
	// N-th roots of unity.
	hint := new(big.Int)
	for i := 1; i < 2*N-1; i += 2 {
		hint.Mul(evalsF[i], evalsG[i])
		hint.Mod(hint, mod)
		prg.Share(mod, hint)
	}
}

func RandomRequest(cfg *config.Config, leaderForReq int) []*ClientRequest {
	//utils.PrintTime("Initialize")
	nf := len(cfg.Fields)
	ns := cfg.NumServers()
	prg := share.NewGenPRG(ns, leaderForReq)

	out := make([]*ClientRequest, ns)
	for s := 0; s < ns; s++ {
		out[s] = new(ClientRequest)
	}
	//utils.PrintTime("ShareData")

	inputs := make([]*big.Int, 0)
	for f := 0; f < nf; f++ {
		field := &cfg.Fields[f]
		switch field.Type {
		default:
			panic("Unexpected type!")
		case config.TypeSM:
			inputs = append(inputs, int_NewRandom(int(field.SMBits))...)
		case config.TypeInt:
			inputs = append(inputs, int_NewRandom(int(field.IntBits))...)
		case config.TypeIntPow:
			inputs = append(inputs, intPow_NewRandom(int(field.IntBits), int(field.IntPow))...)
		case config.TypeIntUnsafe:
			inputs = append(inputs, intUnsafe_NewRandom(int(field.IntBits))...)
		case config.TypeBoolOr:
			inputs = append(inputs, bool_NewRandom()...)
		case config.TypeBoolAnd:
			inputs = append(inputs, bool_NewRandom()...)
		case config.TypeCountMin:
			inputs = append(inputs, countMin_NewRandom(int(field.CountMinHashes), int(field.CountMinBuckets))...)
		case config.TypeLinReg:
			inputs = append(inputs, linReg_NewRandom(field)...)
		}
	}

	//[yoza] Hardcode input

	//inputs = []*big.Int{big.NewInt(23)}

	log.Printf("[yoza] input value is: %v", inputs)

	// Evaluate the Valid() circuit
	ckt := configToCircuit(cfg)
	ckt.Eval(inputs)

	// Generate sharings of the input wires and the multiplication gate wires
	ckt.ShareWires(prg)

	// Construct polynomials f, g, and h and share evaluations of h
	sharePolynomials(ckt, prg)

	log.Printf("[yoza] prg value is: %v", prg)

	triples := triple.NewTriple(share.IntModulus, ns)
	for s := 0; s < ns; s++ {
		out[s].Hint = prg.Hints(s)
		out[s].TripleShare = triples[s]
	}

	return out
}
