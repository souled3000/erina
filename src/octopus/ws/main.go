package main

import (
	"encoding/hex"
	"flag"
	"github.com/golang/glog"
	"net/http"
	"octopus/websocket"
	"os"
	"time"
)

func main() {
	flag.Parse()
	glog.Infoln(os.Args)
	glog.Infoln(os.Args[2])
	LaunchWS(os.Args[2])
}
func LaunchWS(me string) {
	var err error
	var handler func(*websocket.Conn)
	httpServeMux := http.NewServeMux()
	handler = dwh
	wsHandler := websocket.Server{
		Handshake: func(config *websocket.Config, r *http.Request) error {
			if len(config.Protocol) > 0 {
				config.Protocol = config.Protocol[:1]
			}
			return nil
		},
		Handler:  handler,
		MustMask: false,
	}

	httpServeMux.Handle("/ws", wsHandler)
	httpServeMux.Handle("/", wsHandler)
	server := &http.Server{
		//		Addr:        "192.168.2.13:1771",
		Addr:        me,
		Handler:     httpServeMux,
		ReadTimeout: 100 * time.Second,
	}

	glog.Infof("I am %s", me)

	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func dwh(ws *websocket.Conn) {
	glog.Infof("DWH %s", ws.Request().RemoteAddr)
	var (
		err error
		req = make([]byte, 1024)
	)
	x, _ := hex.DecodeString("c200808d74a20f0074a20f0074a20f0074a2bfd5e9c1fad5f2a2cc004ca24cef6dbbb26b25de6dadd96ee24885dd410915997629300cb3d88fddecaec5afd2f970a94a15de2693839b9560695cf4bfe287d374d7215b1f5ae11c550aab034a9d")
	go func() {
		for {
			glog.Infof("MAIN RECV")
			if err = websocket.Message.Receive(ws, &req); err != nil {
				glog.Errorf("[dw:err]%s %v", ws.Request().RemoteAddr, err)
				break
			}
			_, _ = ws.Write(x)
			glog.Infof("<-%s length:%d", ws.Request().RemoteAddr, len(req))

		}
	}()
	go func() {
		glog.Infof("MAIN SEND")
		for i := 0; i < 10000; i++ {
			n, e := ws.Write(x)
			if e != nil {
				ws.Close()
				break
			}
			glog.Infof("->%s length:%d %v", ws.Request().RemoteAddr, n, e)
		}
	}()

	a := make(chan int, 2)
	<-a
}
