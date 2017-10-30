package main

import (
	"fmt"
)

func main() {
	var x [5]byte
	x[1] = 4
	f(x)
	fmt.Printf("%d\n", x[1])

	var y []byte = make([]byte, 3)
	y[1] = 4
	f1(y)
	fmt.Printf("%d\n", y[1])

}

func f(x [5]byte) {
	x[1] = 30
	fmt.Printf("%d\n", x[1])
}

func f1(x []byte) {
	x[1] = 30
	fmt.Printf("%d\n", x[1])
}
