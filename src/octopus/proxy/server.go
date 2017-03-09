package main

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"net"
	"octopus/msgs"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

const (
	kMaxPackageSize = 10240
)

var (
	md5Ctx = md5.New()
)

func begmd5(ctn []byte) []byte {
	defer md5Ctx.Reset()
	md5Ctx.Write(ctn)
	cipher := md5Ctx.Sum(nil)
	return cipher
}

type (
	Server interface {
		Send(peer *net.UDPAddr, msg []byte) // send msg to client/device
		RunLoop() bool
		GetProxySeriveAddr() string
	}

	myServer struct {
		isActive bool
		addr     string
		socket   *net.UDPConn
		socketMu *sync.Mutex
	}
)

func NewServer(addr string) Server {
	return &myServer{
		addr:     addr,
		socketMu: &sync.Mutex{},
	}
}

func (this *myServer) RunLoop() bool {
	if this.isActive {
		return false
	}

	localAddr, err := net.ResolveUDPAddr("udp", this.addr)
	if err != nil {
		glog.Fatalf("Resolve server addr failed: %v", err)
	}

	socket, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		glog.Fatalf("Listen on addr failed: %v", err)
	}
	this.socket = socket

	glog.Infof("Proxy started on %v", socket.LocalAddr())
	input := make([]byte, kMaxPackageSize)
	for {
		n, peer, err := socket.ReadFromUDP(input)
		request := input[:n]
		if glog.V(4) {
			glog.Infof("LOOK!!!：%v,%v,%v", peer.String(), hex.EncodeToString(request), n)
		}
		if err != nil {
			if nerr, ok := err.(*net.OpError); ok && !nerr.Temporary() {
				glog.Errorf("[udp|received] Read failed: %v", nerr)
			}
			continue
		}
		if n < 56 {
			glog.Errorf("This request is too short to handle. %x", request)
			continue
		}
		m := &msgs.Msg{}
		m.Final = request
		err = m.Binary2Msg()
		if err != nil {
			glog.Errorf("%v\n", err)
			continue
		}
		//		timestamp:=p[4:8]
		pMac := make([]byte, 8)
		binary.LittleEndian.PutUint64(pMac, uint64(m.FHSrcId))
		if len(m.Text) < 17 {
			glog.Infof("This mac is too short to handle. %x", m.Text)
			continue
		}
		pMacMd5 := m.Text[:16]
		//		macmd5 := begmd5(pMac)
		macmd5 := md5.Sum(pMac)

		t := m.Text[16]

		//校验mac
		if hex.EncodeToString(pMacMd5) != hex.EncodeToString(macmd5[:]) {
			glog.Errorf("Mac's md5 checking failed, Mac:%v,%v", hex.EncodeToString(pMac), hex.EncodeToString(pMacMd5))
			continue
		}
	again:
		time.Sleep(100 * time.Millisecond)
		output, adr := ChooseAUDPServer(pMac)
		switch t {
		case 1:
			if !strings.HasPrefix(adr, "udp") {
				goto again
			}
		case 2:
			if !strings.HasPrefix(adr, "tcp") {
				goto again
			}
		default:
			glog.Error("unrecognized :%d", t)
			continue
		}

		if adr != "" {
			go this.Send(peer, output)
			if glog.V(4) {
				glog.Infof("RETURN:%v %v %v %x", m.FHSrcMac, peer.String(), adr, output)
			}
		}
	}
	return true
}

func (this *myServer) Send(peer *net.UDPAddr, msg []byte) {
	this.socketMu.Lock()
	defer this.socketMu.Unlock()
	n, err := this.socket.WriteToUDP(msg, peer)
	if err != nil {
		glog.Errorf("[udp|sended] peer: %v, msg: len(%d)%v,err:%v", peer.String(), n, hex.EncodeToString(msg), err)
	}
}

func (this *myServer) GetProxySeriveAddr() string {
	return this.addr
}
