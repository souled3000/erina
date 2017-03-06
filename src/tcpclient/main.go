package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	adr, _ := net.ResolveTCPAddr("tcp", os.Args[1])
	con, _ := net.DialTCP("tcp", nil, adr)
	a, _ := hex.DecodeString("c2000f032e6a9c019c012e6a9c012e6a9c019ebf0162d6759201ea6a840151f4699fae56b4b84642f9f89811e84758ae89b11f34c785ced170c525d6c4056d60")
	fmt.Println(len(a))
	for {
		con.Write(a)
		time.Sleep(1 * time.Millisecond)
	}
}
