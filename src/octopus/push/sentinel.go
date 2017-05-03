package main

import (
	"encoding/binary"
	"github.com/golang/glog"
	"net"
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
	IP   string
	Adr  string
	Con  *net.TCPConn
	Type int
}
type Monitor struct {
	Adr *string
}

func (this *SentinelMgr) doesExsist(id int64) bool {
	var b bool
	var srv string
//	var t int
//	if id > 0 {
//		t = 0
//	} else {
//		t = 1
//	}
	this.mu.RLock()
	defer this.mu.RUnlock()
	if _, ok := this.sentinels[this.usr2sentinel[id]]; ok {
		return true
	} else {
		return false
//		srv := getSrvById(id)
//		if srv != "" {
//			if s, ok := this.sentinels[srv]; ok {
//				_, err := s.Con.Write([]byte{0x00, 0x00, 0x00, 0x00})
//				if err == nil {
//					b = true
//				} else {
//					if this.add(srv, t) {
//						b = true
//					} else {
//						delete(this.sentinels, srv)
//					}
//				}
//			} else {
//				if this.add(srv, t) {
//					b = true
//				}
//			}
//		}
	}
	if b {
		this.usr2sentinel[id] = srv
	}
	return b
}
func (this *SentinelMgr) Forward(ids []int64, msg []byte) {
	for _, id := range ids {
		var p []byte = make([]byte, 12+len(msg))
		binary.LittleEndian.PutUint32(p[0:4], uint32(8+len(msg)))
		binary.LittleEndian.PutUint64(p[4:12], uint64(id))
		copy(p[12:], msg[:])
		this.mu.RLock()
		defer this.mu.RUnlock()
		if sentinel, ok := this.sentinels[this.usr2sentinel[id]]; ok {
			_, err := sentinel.Con.Write(p)
			if err != nil {
				//				pushOfflineMsg(id, msg)//推送失败记录离线消息
				glog.Errorf("[STL] FAIL DST:%v CTN:%x ER:%v", id, msg, err)

				sentinel.Con.Close()
				adr, e := net.ResolveTCPAddr("tcp", sentinel.Adr)
				sentinel.Con, e = net.DialTCP("tcp", nil, adr)
				if e == nil {
					glog.Infof("REBUILD_S:%s", sentinel.Adr)
					_,err=sentinel.Con.Write(p)
				} else {
					glog.Infof("REBUILD_F:%s %v", sentinel.Adr, e)
					time.Sleep(time.Millisecond)
				}
			} else {
				glog.Infof("[STL] SUCC DST:%v CTN:%x", id, msg)
			}
		} else {
			//			pushOfflineMsg(id, msg)//目标不在线，记录离线消息
			if glog.V(5) {
				glog.Infof("[STL] FAIL DST:%v Not Found.", id)
			}
		}
	}
}
func (this *SentinelMgr) add(leaf string, t int) bool {
	tcpAdr, e := net.ResolveTCPAddr("tcp", leaf)
	con, e := net.DialTCP("tcp", nil, tcpAdr)
	if e == nil {
		node := &Sentinel{
			Con:  con,
			Adr:  leaf,
			IP:   tcpAdr.IP.String(),
			Type: t,
		}
		this.mu.Lock()
		this.sentinels[leaf] = node
		this.mu.Unlock()
		if glog.V(5) {
			glog.Infof("[STL] Connected %v,%v", leaf, node)
		}
		return true
	} else {
		glog.Errorf("[STL] %v Fail To Connected. %v", leaf, e)
		return false
	}
}
func (this *SentinelMgr) update(leaves map[string]string, t int) {
	this.mu.Lock()
	defer this.mu.Unlock()
	for leaf, _ := range leaves {
		if _, ok := this.sentinels[leaf]; !ok {
			tcpAdr, e := net.ResolveTCPAddr("tcp", leaf)
			con, e := net.DialTCP("tcp", nil, tcpAdr)
			if e == nil {
				node := &Sentinel{
					Con:  con,
					Adr:  leaf,
					IP:   tcpAdr.IP.String(),
					Type: t,
				}
				this.sentinels[leaf] = node
				if glog.V(5) {
					glog.Infof("[STL] Connected %v", leaf)
				}
			} else {
				glog.Errorf("[STL] %v Fail To Connect. %v", leaf, e)
			}
		}
	}
	for leaf, sentinel := range this.sentinels {
		if sentinel.Type != t {
			continue
		}
		if _, ok := leaves[leaf]; !ok {
			sentinel.Con.Close()
			delete(this.sentinels, leaf)
			if glog.V(5) {
				glog.Infof("[STL] Disconnected %v", leaf)
			}
		}
	}
}
func (this *SentinelMgr) Cancel(s *Sentinel) {
	this.mu.Lock()
	defer this.mu.Unlock()
	s.Con.Close()
	delete(this.sentinels, s.IP)
}
