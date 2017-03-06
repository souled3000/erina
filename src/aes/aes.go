package main

import (
	"encoding/hex"
	"fmt"
	aes "octopus/common/aes"
)

func main() {
	k := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	txt, _ := hex.DecodeString("3b7b1f4aeec54ce150140257695cf794")
	ci, _ := aes.EncryptNp(txt, k)
	fmt.Printf("%x\n", ci)

	txt, _ = aes.DecryptNp(ci, k)
	fmt.Printf("%x", txt)
}
