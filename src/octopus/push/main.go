package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	p "github.com/magiconair/properties"
	"octopus/ver"
	"strings"
)

var (
	UdpPath      string
	WsPath       string
	GSentinelMgr = NewSentinelMgr()
)

func main() {
	cfg := p.MustLoadFile("push.properties", p.UTF8)
	rh := cfg.GetString("redis", "127.0.0.1:6379")
	zks := cfg.GetString("zks", "")
	UdpPath = cfg.GetString("uc", "UdpComet")
	WsPath = cfg.GetString("wc", "WsComet")
	printVer := flag.Bool("ver", false, "Comet版本")
	flag.Parse()
	if *printVer {
		fmt.Printf("Comet %s, 插座后台代理服务器.\n", ver.Version)
		return
	}
	defer glog.Flush()
	InitZK(strings.Split(zks, ","), UdpPath, WsPath)
	InitModel(rh)
	InitUsr2Sentinel()
	handleSignal(func() {
		CloseZK()
		glog.Info("MsgBusServer Closed")
	})
}
