package srv

import (
	"fmt"
	"github.com/golang/glog"
	"net"
	"octopus/msgs"
)

const (
	kMaxPackageSize = 10240
)

type Datagram struct {
	adr  *net.UDPAddr
	data *[]byte
	aim  int64
	mac  string
}
type UdpServer struct {
	Addr  string
	Con   *net.UDPConn
	Queue chan *Datagram
}
type DevRequest struct {
	Peer *net.UDPAddr
	Msg  *msgs.Msg
}

func (this *UdpServer) sender() {
	this.Queue = make(chan *Datagram, 100000)
	go func() {
		var e error
		for d := range this.Queue {
			_, e = this.Con.WriteToUDP(*((*d).data), d.adr)
			if e != nil {
				glog.Errorf("DAMN. %s|%x|%v", d.adr.String(), *d.data, e)
			} else {
				if glog.V(5) {
					glog.Infof("[DONE:%v:%s:%s]%x", d.aim, d.mac, d.adr.String(), *(d.data))
				}
			}
		}
	}()
	glog.Infof("Launch UdpSrv Sender")
}
func (s *UdpServer) RunLoop() {
	localAddr, err := net.ResolveUDPAddr("udp", s.Addr)
	if err != nil {
		glog.Fatalf("Resolve server addr failed: %v,%v", err, s.Addr)
	}
	s.Con, err = net.ListenUDP("udp", localAddr)
	if err != nil {
		glog.Fatalf("Listen on addr failed: %v", err)
	}
	s.sender()

	glog.Infof("UdpSrv started on %v\n\n", s.Con.LocalAddr())

	input := make([]byte, kMaxPackageSize)
	for {
		n, peer, err := s.Con.ReadFromUDP(input)
		if err != nil {
			if nerr, ok := err.(*net.OpError); ok && !nerr.Temporary() {
				glog.Fatalf("%v", nerr)
			}
			continue
		}
		if glog.V(3) {
			glog.Infof("UdpSrv RECEIVED!!!!!! %v,len(%d)%x", peer, n, input[:n])
		}
		msgCopy := make([]byte, n)
		copy(msgCopy, input[0:n])
		go s.Process(peer, msgCopy)
	}
}

func (s *UdpServer) Send(peer *net.UDPAddr, msg *msgs.Msg) {
	data := &Datagram{data: &msg.Final, adr: peer, aim: msg.FHDstId, mac: msg.FHDstMac}
	s.Queue <- data
}
func (s *UdpServer) Send2(peer *net.UDPAddr, msg *msgs.Msg, sess *DevSession, busi string) {
	if glog.V(3) {
		glog.Infof("[udp|ret %v] %s, response:%s", busi, sess, msg.String())
	}
	data := &Datagram{data: &msg.Final, adr: peer, aim: sess.devId, mac: sess.mac}
	s.Queue <- data
}

func (s *UdpServer) Process(peer *net.UDPAddr, input []byte) {
	m := &msgs.Msg{}
	m.Final = input
	t := &DevRequest{
		Peer: peer,
		Msg:  m,
	}
	mac, err := s.handle(t)
	if err != nil && glog.V(5) {
		glog.Errorf("[ERROR!!] %s , req(len[%d]，%x)，%v", mac, len(t.Msg.Final), t.Msg.Final, err)
	}
}

func (server *UdpServer) handle(t *DevRequest) (string, error) {
	m := t.Msg
	m.Header()
	var (
		sess *DevSession
		mac  string
		err  error
		busi string
		ret  *msgs.Msg
	)
	switch {
	case m.FHOpcode == 0x02:
		sess, mac, err, busi, ret = opcode02(t, nil)
		if err == nil {
			if sess != nil {
				server.Send2(t.Peer, ret, sess, busi)
			} else {
				server.Send(t.Peer, ret)
			}
		}
	case m.FHOpcode == 0x03:
		sess, mac, err = opcode03(m, nil)
	case m.FHOpcode == 0x05:
		sess, mac, err = opcode05(m, nil)
	default:
		return m.FHSrcMac, fmt.Errorf("err: wrong opcode %d", m.FHOpcode)
	}
	if sess != nil {
		sess.Update(t.Peer)
	}
	return mac, err
}
