package srv

import (
	"bufio"
	"encoding/binary"
	"github.com/golang/glog"
	"io"
	"net"
	"octopus/msgs"
	"sync"
	"time"
)

type SentinelMgr struct {
	sentinels    map[string]*Sentinel
	mu           *sync.RWMutex
	usr2sentinel map[int64]string
}

func NewSentinelMgr() *SentinelMgr {
	return &SentinelMgr{sentinels: make(map[string]*Sentinel), mu: &sync.RWMutex{}, usr2sentinel: make(map[int64]string)}
}

type Sentinel struct {
	IP    string
	Adr   string
	Aim   *net.TCPAddr
	w     *bufio.Writer
	Con   *net.TCPConn
	Type  int
	Queue chan *[]byte
}

func (this *SentinelMgr) add(leaf string, t int) bool {
	tcpAdr, e := net.ResolveTCPAddr("tcp", leaf)
	con, e := net.DialTCP("tcp", nil, tcpAdr)
	if e == nil {
		node := &Sentinel{
			Con:  con,
			w:    bufio.NewWriter(con),
			Adr:  leaf,
			Aim:  tcpAdr,
			IP:   tcpAdr.IP.String(),
			Type: t,
		}
		node.monitor()
		this.mu.Lock()
		defer this.mu.Unlock()
		this.sentinels[leaf] = node
		glog.Infof("[STL] Connected %v,%v", leaf, node)
		return true
	} else {
		glog.Errorf("[STL] %v Fail To Connected. %v", leaf, e)
		return false
	}
}
func (this *SentinelMgr) doesExsist(id int64) bool {
	var b bool
	var t int
	var srv string
	if id > 0 {
		t = 0
	} else {
		t = 1
	}
	this.mu.Lock()
	defer this.mu.Unlock()
	if peer, ok := this.sentinels[this.usr2sentinel[id]]; ok && peer.Con != nil {
		return true
	} else {
		srv := getSrvById(id)
		if srv != "" {
			if s, ok := this.sentinels[srv]; ok {
				_, err := s.Con.Write([]byte{0x00, 0x00, 0x00, 0x00})
				if err == nil {
					b = true
				} else {
					if this.add(srv, t) {
						b = true
						if glog.V(5) {
							glog.Infof("[STL] %s rebuilded", srv)
						}
					} else {
						delete(this.sentinels, srv)
					}
				}
			} else {
				if this.add(srv, t) {
					b = true
					if glog.V(5) {
						glog.Infof("[STL] %s rebuilded", srv)
					}
				}
			}
		}
	}
	if b {
		this.usr2sentinel[id] = srv
	} else {
		if glog.V(5) {
			glog.Infof("[STL] %d unreachable", id)
		}
	}
	return b
}

func (this *SentinelMgr) Forward(ids []int64, msg []byte) {
	this.mu.Lock()
	defer this.mu.Unlock()
	for _, id := range ids {
		//		if !this.doesExsist(id) {
		//			continue
		//		}

		if sentinel, ok := this.sentinels[this.usr2sentinel[id]]; ok {

			var p []byte = make([]byte, 12+len(msg))
			length := 8 + len(msg)
			if length > 1024 {
				glog.Errorf("[STL] request is too long. length is %d", length)
				return
			}
			binary.LittleEndian.PutUint32(p[0:4], uint32(length))
			binary.LittleEndian.PutUint64(p[4:12], uint64(id))
			copy(p[12:], msg[:])

			//			sentinel.Queue <- &p
		repeat:
			_, err := sentinel.Con.Write(p)
			if err != nil {
				glog.Error(err)
				peeradr, err := net.ResolveTCPAddr("tcp", sentinel.Adr)
				glog.Error(err)
				sentinel.Con, err = net.DialTCP("tcp", nil, peeradr)
				glog.Error(err)
				if err == nil {
					glog.Infof("STL restore.%v", err)
					goto repeat
				}
			}
			if glog.V(5) {
				glog.Infof("[STL] DST:%v ENQUEUE %v", id,err)
			}
		} else {
			if glog.V(5) {
				glog.Infof("[STL] %v Not Found. Forward Fail", id)
			}
		}
	}
	statIncUpStreamOut()
}
func (this *Sentinel) monitor() {
	go func() {
		if glog.V(5) {
			glog.Infof("STL %s MONITOR BEGGING", this.Adr)
		}
		defer func() {
			if glog.V(5) {
				glog.Infof("STL %s MONITOR ENDDING", this.Adr)
			}
		}()
		var e error
		for d := range this.Queue {
		again:
			_, e = this.w.Write(*d)
			if e != nil {
				glog.Errorf("STL SEER:%v", e)
				goto again
			} else {
				e = this.w.Flush()
				if glog.V(3) {
					glog.Infof("STL DEQUEUE:%x %v", *d, e)
				}
			}
		}

	}()
}

type Monitor struct {
	Adr *string
}

func (this *Monitor) LaunchMonitor() {
	GSentinelMgr = NewSentinelMgr()
	laddr, _ := net.ResolveTCPAddr("tcp", *GMonitor.Adr)
	listener, _ := net.ListenTCP("tcp", laddr)
	glog.Infof("STLSRV Started On %v", *GMonitor.Adr)
	go func() {
		for {
			conn, _ := listener.AcceptTCP()
			go func() {
				var err error
				if glog.V(5) {
					glog.Infof("STLCLI START %v", conn.RemoteAddr().String())
				}
				defer func() {
					if glog.V(5) {
						glog.Infof("STLCLI END %v,%v", conn.RemoteAddr().String(), err)
					}
				}()
				defer conn.Close()
				h4 := make([]byte, 4)
				for {

					n, err := io.ReadFull(conn, h4)
					if n == 0 || err != nil {
						glog.Errorf("STL [%s] err: %v || len=%d", conn.RemoteAddr().String(), err, n)
						break
					}

					// read payload, the size of the payload is given by h4
					size := binary.LittleEndian.Uint32(h4)
					if size == 0 {
						glog.Errorf("STL error: msg length is 0")
						continue
					}
					data := make([]byte, size)
					n, err = io.ReadFull(conn, data)
					if err != nil {
						glog.Errorf("STL error receiving payload:%s", err)
						break
					}
					id := int64(binary.LittleEndian.Uint64(data[0:8]))
					if glog.V(5) {
						glog.Infof("STL-RECV: dst:%d ctn:%x", id, data[8:])
					}
					switch SrvType {
					case CometWs:
						UsrSessions.PushMsg(id, data[8:])
					case CometUdp:
						m := &msgs.Msg{}
						m.Final = data[8:]
						m.Binary2Msg()
						if m.FHOpcode == 0x02 && m.DHMsgId == 0xc7 {
							dst := int64(binary.LittleEndian.Uint64(m.Text[0:8]))
							if ses, ok := DevSessions.Sesses[dst]; ok {
								if dst == ses.devId {
									ses.offlineEvent.Reset(0 * time.Second)
									glog.Infof("STL MAIN kill %d", dst)
								} else {
									subdevlogout(ses, dst)
								}
							} else {
								glog.Infof("STL FAIL TO kill %d", dst)
							}
							continue
						}
						DevSessions.PushMsg(id, data[8:])
					case CometDw:
						DevSessions.PushMsg(id, data[8:])
					}
				}
			}()
		}
	}()

}

func (this *SentinelMgr) update(leaves map[string]string, t int) {
	this.mu.Lock()
	defer this.mu.Unlock()
	for leaf, _ := range leaves {
		if leaf == *SentinelAdr {
			continue
		}
		if _, ok := this.sentinels[leaf]; !ok {
			tcpAdr, e := net.ResolveTCPAddr("tcp", leaf)
			con, e := net.DialTCP("tcp", nil, tcpAdr)
			if e == nil {
				node := &Sentinel{
					Con: con,
					//					w:     bufio.NewWriter(con),
					Adr:   leaf,
					IP:    tcpAdr.IP.String(),
					Queue: make(chan *[]byte, 100000),
					Type:  t,
				}
				node.monitor()
				this.sentinels[leaf] = node
				if glog.V(5) {
					glog.Infof("connected %v,%v", leaf, node)
				}
			} else {
				glog.Errorf("Dial %v Fail. %v", leaf, e)
			}
		} else {
			if glog.V(5) {
				glog.Infof("%v Exsisted", leaf)
			}
		}
	}
	for leaf, sentinel := range this.sentinels {
		if sentinel.Type != t {
			continue
		}
		if _, ok := leaves[leaf]; !ok {
			close(sentinel.Queue)
			sentinel.Con.Close()
			delete(this.sentinels, leaf)
			if glog.V(5) {
				glog.Infof("disconnected %v", leaf)
			}
		} else {
			if glog.V(5) {
				glog.Infof("%v Exsisted", leaf)
			}
		}
	}
}

func (this *SentinelMgr) NotifyBindedIdChanged(deviceId int64, newBindIds []int64, unbindIds []int64) {
	// new code
	body := msgs.MsgStatus{}
	body.Id = deviceId
	m := &msgs.Msg{}
	m.FHSrcId = deviceId
	m.DHRead = true
	m.FHMask = true
	m.FHOpcode = 2
	m.DHMsgId = msgs.MIDStatus
	if len(newBindIds) > 0 {
		body.Type = msgs.MSTBinded
		m.Text, _ = body.Marshal()
		m.Msg2Binary()
		GSentinelMgr.Forward(newBindIds, m.Final)
	}
	if len(unbindIds) > 0 {
		body.Type = msgs.MSTUnbinded
		m.Text, _ = body.Marshal()
		m.Msg2Binary()
		GSentinelMgr.Forward(unbindIds, m.Final)
	}

}
