package mpc

import (
	"log"
	"math/big"
	//"bufio"
	//"strconv"
	//"os"
	"../share"

	"../circuit"
	"../utils"
)

//var random_nums []int64
var num_index int
var localAgg *big.Int

func init() {

	num_index = 0

	localAgg = big.NewInt(0)
/*
	file, err := os.Open("/root/file.txt")
	if err != nil {
		log.Fatal("Error in opening file")
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		x, err := strconv.Atoi(scanner.Text())
		if err != nil {
			log.Fatal("Error reading from file")
		}

		random_nums = append(random_nums, int64(x))
	}
*/

}

func bigToBits(nBits int, value *big.Int) []*big.Int {

	if value.Cmp(utils.SMMax) == 1{
		log.Printf("value: %v not added to localAgg", value)
	} else {
		localAgg.Add(localAgg, value)
		localAgg.Mod(localAgg, share.IntModulus)
		log.Printf("Values addes to localAgg; value: %v, localAgg: %v", value, localAgg)
	}

	bits := make([]*big.Int, nBits)
	for i := 0; i < nBits; i++ {
		bits[i] = big.NewInt(int64(value.Bit(i)))
	}

	return bits
}

func int_Circuit(name string, nBits int) *circuit.Circuit {
	return circuit.NBits(nBits, name)
}

func SM_Circuit(name string, nBits int) *circuit.Circuit {
	return circuit.SM_NBits(nBits, name)
}

func int_NewRandom(nBits int) []*big.Int {
	max := big.NewInt(1)
	max.Lsh(max, uint(nBits-2))
	//[yoza] hardcode value
	v := utils.RandInt(max)
	//v := big.NewInt(random_nums[num_index])
	//num_index = num_index + 1
	log.Printf("[yoza] new random int is : %d", v)
	return int_New(nBits, v)
}

func int_New(nBits int, value *big.Int) []*big.Int {
	if nBits < 1 {
		log.Fatal("nBits must have value >= 1")
	}

	if value.Sign() == -1 {
		log.Fatal("Value must be non-negative")
	}

	vLen := value.BitLen()
	if vLen > nBits {
		log.Fatal("Value is too long")
	}

	return bigToBits(nBits, value)
}
