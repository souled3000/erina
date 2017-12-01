package main

import (
	"crypto/md5"
	"fmt"
)

func main() {
	var a []byte = make([]byte, 16)
	var b []byte = make([]byte, 16)
	m := make(map[[16]byte]int)
	amd5 := md5.Sum(a)
	bmd5 := md5.Sum(b)
	m[amd5] = 1
	if _, ok := m[bmd5]; ok {
		fmt.Println("ok")
	} else {
		fmt.Println("nk")
	}

}
