package main

import (
	"cloud-base/atomic"
	"cloud-base/zk"
	//	"encoding/binary"
	//	"fmt"
	"github.com/golang/glog"
	zookeeper "github.com/samuel/go-zookeeper/zk"
	//	"io"
	//	"io/ioutil"
	//	"net"
	//	"octopus/msgs"
	"strings"
	"time"
)

var (
	zkConn   *zookeeper.Conn
	zkConnOk atomic.AtomicBoolean
	//zkReportCh	chan cometStat
	zkReportCh  chan zookeeper.Event
	zkCometRoot string
)

type cometStat struct {
	Ok   bool
	Path string
	Url  string
}

func init() {
	zkReportCh = make(chan zookeeper.Event, 1)
	zkConnOk.Set(false)
}

func onConnStatus(event zookeeper.Event) {
	switch event.Type {
	case zookeeper.EventSession:
		switch event.State {
		case zookeeper.StateHasSession:
			zkConnOk.Set(true)
		case zookeeper.StateDisconnected:
			fallthrough
		case zookeeper.StateExpired:
			zkConnOk.Set(false)
		}
	}
	zkReportCh <- event
}

func InitZK(zkAddrs []string, udpPath string, wsPath string) {
	if len(udpPath) == 0 {
		glog.Fatalf("[zk] root name for msgbus cannot be empty")
	}
	if len(wsPath) == 0 {
		glog.Fatalf("[zk] root name for comet cannot be empty")
	}
	var (
		nodes []string
		err   error
		addr  string
		watch <-chan zookeeper.Event
	)
	zkConn, err = zk.Connect(zkAddrs, 60*time.Second, onConnStatus)
	if err != nil {
		glog.Fatal(err)
	}
	go func() {
		glog.Infof("Watching %s", WsPath)
		for {
			nodes, _, watch, err = zkConn.ChildrenW("/" + WsPath)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			leaves := make(map[string]string, len(nodes))
			for _, node := range nodes {
				addr, err = zk.GetNodeData(zkConn, "/"+WsPath+"/"+node)
				if err != nil {
					glog.Errorf("[%s] cannot get", addr)
					continue
				}
				if glog.V(5){
					glog.Infof("WsNode:%v Value:%v Size：%d", node, addr,len(strings.Split(addr, ",")))
				}
				addr = strings.Split(addr, ",")[5]
				leaves[addr] = ""
			}
			GSentinelMgr.update(leaves, 0)
			<-watch
			if glog.V(5) {
				glog.Infof("WsPath has changed")
			}
			time.Sleep(time.Second)
		}
	}()
	go func() {
		glog.Infof("Watching %s", UdpPath)
		for {
			nodes, _, watch, err = zkConn.ChildrenW("/" + UdpPath)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			leaves := make(map[string]string, len(nodes))
			for _, node := range nodes {
				addr, err = zk.GetNodeData(zkConn, "/"+UdpPath+"/"+node)
				if err != nil {
					glog.Errorf("[%s] cannot get", addr)
					continue
				}
				if glog.V(5) {
					glog.Infof("UdpNode:%v Value:%v Size:%d", node, addr, len(strings.Split(addr, ",")))
				}
				tmp := strings.Split(addr, ",")[5]
				leaves[tmp] = ""
			}
			GSentinelMgr.update(leaves, 1)
			<-watch
			glog.Infof("UdpPath has changed")
			time.Sleep(time.Second)
		}
	}()
}
func CloseZK() {
	if zkConn != nil {
		zkConn.Close()
	}
}
