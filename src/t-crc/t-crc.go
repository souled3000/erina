package main

import (
	"fmt"
	"octopus/common"
)

func f() {
	d := []byte("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz")
	r := common.Crc(d, len(d))
	fmt.Println(r)
}
func main() {
	f()
}
