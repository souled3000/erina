package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	adr, _ := net.ResolveTCPAddr("tcp", os.Args[1])
	srv, _ := net.ListenTCP("tcp", adr)
	for {
		client, _ := srv.AcceptTCP()
		go func(c *net.TCPConn) {
			m := make([]byte, 64)
			for {
				n, e := c.Read(m)
				fmt.Printf("R:%s %d %x %v\n", c.RemoteAddr().String(), n, m[0:n], e)
				if e != nil {
					fmt.Println(e)
					c.Close()
					break
				}
			}
		}(client)
	}
}

func f() {
	adr, _ := net.ResolveTCPAddr("tcp", os.Args[1])
	srv, _ := net.ListenTCP("tcp", adr)
	for {
		client, _ := srv.AcceptTCP()
		go func(c *net.TCPConn) {
			m := make([]byte, 64)
			for {
				n, e := c.Read(m)
				fmt.Printf("R:%s %d %x %v\n", c.RemoteAddr().String(), n, m[0:n], e)
				if e != nil {
					fmt.Println(e)
					c.Close()
					break
				}
			}
		}(client)
	}
}

func f2() {

}
