package main

import (
	"fmt"
	"math/rand"
	"time"
	"unsafe"
)

const (
	testsetSize = 32 * 1024
	testRounds  = 512 * 1024
)

type config0 uint64

type config1 struct {
	f1 uint16
	f2 uint16
	f3 uint16
	f4 uint16
}

type config2 struct {
	f1 uint16
	f2 uint16
	f3 uint16
	f4 [2]uint8
}

type config3 struct {
	f1 uint32
	f2 bool
	f3 bool
	f4 uint8
	f5 uint8
}

func printResult(tag string, elapsed time.Duration) {
	numOps := testsetSize * testRounds
	fmt.Printf("%10s: %11d ops in %10v -> %d millions lookups/s\n", tag, numOps, elapsed, numOps/int(elapsed.Milliseconds())/1000)
}

func testConfig0(idx []int) {
	data := make([]config0, testsetSize)

	for i := 0; i < testsetSize; i++ {
		data[i] = config0(i)
	}

	start := time.Now()

	for i := 0; i < testRounds; i++ {
		for j := 0; j < testsetSize; j++ {
			_ = data[idx[j]]
		}
	}
	elapsed := time.Now().Sub(start)

	printResult("config0", elapsed)
}

func testConfig1(idx []int) {
	data := make([]config1, testsetSize)

	for i := 0; i < testsetSize; i++ {
		data[i] = config1{f1: uint16(i), f2: uint16(i * 2), f3: uint16(i * 3), f4: uint16(i * 5)}
	}

	start := time.Now()

	for i := 0; i < testRounds; i++ {
		for j := 0; j < testsetSize; j++ {
			_ = data[idx[j]]
		}
	}
	elapsed := time.Now().Sub(start)

	printResult("config1", elapsed)

}

func testConfig2(idx []int) {
	data := make([]config2, testsetSize)

	for i := 0; i < testsetSize; i++ {
		data[i] = config2{f1: uint16(i), f2: uint16(i * 2), f3: uint16(i * 3), f4: [2]uint8{uint8(i), uint8(i * 2)}}
	}

	start := time.Now()

	for i := 0; i < testRounds; i++ {
		for j := 0; j < testsetSize; j++ {
			_ = data[idx[j]]
		}
	}
	elapsed := time.Now().Sub(start)

	printResult("config2", elapsed)

}

func testConfig3(idx []int) {
	data := make([]config3, testsetSize)

	for i := 0; i < testsetSize; i++ {
		data[i] = config3{f1: uint32(i), f2: i&3 != 0, f3: i&7 == 0, f4: uint8(i), f5: uint8(i * 3)}
	}

	start := time.Now()

	for i := 0; i < testRounds; i++ {
		for j := 0; j < testsetSize; j++ {
			_ = data[idx[j]]
		}
	}
	elapsed := time.Now().Sub(start)

	printResult("config3", elapsed)

}

func main() {
	rnd := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

	idx := rnd.Perm(testsetSize)

	// Print information
	fmt.Printf("sizeof(%T): %d\n", config0(0), unsafe.Sizeof(config0(9)))
	fmt.Printf("sizeof(%T): %d\n", config1{}, unsafe.Sizeof(config1{}))
	fmt.Printf("sizeof(%T): %d\n", config2{}, unsafe.Sizeof(config2{}))
	fmt.Printf("sizeof(%T): %d\n", config3{}, unsafe.Sizeof(config3{}))

	//  Run tests
	testConfig0(idx)
	testConfig1(idx)
	testConfig2(idx)
	testConfig3(idx)

}
