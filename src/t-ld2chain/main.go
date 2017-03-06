package main

import (
	"encoding/hex"
	"fmt"
	"octopus/common"
)

func main() {
	devs := make([]byte, 4)
	//	macs := []string{"0000000022222222", "0000000011111111"}
	macs := []string{}
	devs[2] = 0x80 | byte(len(macs))
	for _, mac := range macs {
		macbyte, _ := hex.DecodeString(mac)
		devs = append(devs, macbyte[4:8]...)
	}
	devs[0] = common.Crc(devs[1:], len(devs[1:]))
	fmt.Printf("%x", devs)
}
