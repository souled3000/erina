package main

import (
	"encoding/hex"
	"fmt"
	"net"
	//	"os"
	"time"
)

//func main2() {
//	adr, _ := net.ResolveTCPAddr("tcp", os.Args[1])
//	con, _ := net.DialTCP("tcp", nil, adr)
//	a, _ := hex.DecodeString("c2000f032e6a9c019c012e6a9c012e6a9c019ebf0162d6759201ea6a840151f4699fae56b4b84642f9f89811e84758ae89b11f34c785ced170c525d6c4056d60")
//	fmt.Println(len(a))
//	for {
//		a := make([]byte, 10)
//		n, e := con.Read(a)
//		if e != nil {
//			fmt.Println(n, e)
//			break
//		}
//		//		con.Write(a)
//		time.Sleep(1 * time.Millisecond)
//	}
//}

//func main() {
//	adr, _ := net.ResolveTCPAddr("tcp", "localhost:10000")
//	con, _ := net.DialTCP("tcp", nil, adr)
//	a, _ := hex.DecodeString("fac3c6c9053c39364000c2000f032e6a9c019c012e6a9c012e6a9c019ebf0162d6759201ea6a840151f4699fae56b4b84642f9f89811e84758ae89b11f34c785ced170c525d6c4056d60")
//	fmt.Println(len(a))
//	for {
//		n, e := con.Write(a)
//		if e != nil {
//			fmt.Println(n, e)
//			break
//		}
//		//		con.Write(a)
//		//		time.Sleep(1 * time.Millisecond)
//		time.Sleep(1 * time.Second)
//	}
//}

func main() {
	adr, _ := net.ResolveTCPAddr("tcp", "localhost:10000")
	con, _ := net.DialTCP("tcp", nil, adr)
	a, _ := hex.DecodeString("11111111100000fac3c6c9053c39364000c2000f032e6a9c019c012e6a9c012e6a9c019ebf0162d6759201ea6a840151f4699fae56b4b84642f9f89811e84758ae89b11f34c785ced170c525d6c4056d60")
	for i := 0; i < 3; i++ {
		n, e := con.Write(a)
		fmt.Println(i, n, e)
		a := make([]byte, 10)
		n, e = con.Read(a)
		fmt.Println(n, e, fmt.Sprintf("%x", a[0:4]))
		time.Sleep(1 * time.Second)
	}
	time.Sleep(100 * time.Second)
	con.Close()
}
