package main

import (
	"encoding/hex"
	"fmt"
	"octopus/common"
)

func f() {
	d, _ := hex.DecodeString("00000000000000000001")
	r := common.Crc(d, len(d))
	fmt.Println(r)
}
func main() {
	f()
}
