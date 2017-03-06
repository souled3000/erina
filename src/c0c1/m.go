package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

func main() {
	buf := make([]byte, 10240)
	//	S1, _ := net.ResolveUDPAddr("udp", "101.200.76.73:7777")
	//	S1, _ := net.ResolveUDPAddr("udp", "123.57.47.53:7777")
	S1, _ := net.ResolveUDPAddr("udp", "192.168.2.13:7777")
	S2, _ := net.ResolveUDPAddr("udp", "123.57.47.53:1777")
	con, _ := net.ListenUDP("udp", nil)

	bt, _ := hex.DecodeString("c2000200a83a0000a83a0000a83a0000a83ab0d53559f6b82e3ac000b03aec5f72f952552fcde60ae631ef37bdccf9ce0ec6ea20994e7c98985c404a5314a37d")
	//	sess, _ := hex.DecodeString("c2000400f8130ee1f8130ee1f8130ee1e9131fe1e9131fe17e13cfe1c01352bd129468591cd6d49f019364540a67c1eb80826006cb2a686154d510b03a09ce8509f766843eafe44244a01386098d61184145fc0784f0798a840ffdc2c7c17bcf")
	n1 := 0
	n2 := 0
	m := 0

	go func() {
		for {
			_, peer, _ := con.ReadFromUDP(buf)
			if peer.String() == S1.String() {
				n1++
				fmt.Printf("第%d次接收自%s %x... \n", n1, peer.String(), buf[:5])
			}
			if peer.String() == S2.String() {
				n2++
				fmt.Printf("第%d次接收自%s %x... \n", n2, peer.String(), buf[:5])
			}
		}
	}()

	for {
		m++
		con.WriteToUDP(bt, S1)
		//		con.WriteToUDP(sess, S2)
		fmt.Printf(" 第%d次发送 \n", m)
		time.Sleep(10 * time.Second)
	}
}
