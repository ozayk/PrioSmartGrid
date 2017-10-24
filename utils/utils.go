package utils

import "math/big"

var Zero *big.Int
var One *big.Int
var NegOne *big.Int
var SMMax *big.Int
var C_ID int64

func BoolToInt64(b bool) int64 {
	if b {
		return 1
	} else {
		return 0
	}
}

func NextPowerOfTwo(val int) int {
	i := val
	out := uint(0)
	for ; i > 0; i >>= 1 {
		out++
	}

	pow := 1 << out
	if pow > 1 && pow/2 == val {
		return val
	} else {
		return pow
	}
}

func init() {
	Zero = big.NewInt(0)
	One = big.NewInt(1)
	NegOne = big.NewInt(-1)
	SMMax = big.NewInt(512)
}
