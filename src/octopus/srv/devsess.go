package srv

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"net"
	"octopus/msgs"
	"octopus/websocket"
	"strings"
	"sync"
	"time"
	//	"unsafe"
	gw "github.com/gorilla/websocket"
)

type DevSession struct {
	sid           string
	devId         int64        `json:"DeviceID"`
	udpAdr        *net.UDPAddr `json:"Addr"`
	lastHeartbeat time.Time    `json:"LastHeartbeat"`
	owner         int64
	mac           string
	users         []int64
	devs          map[int64]string
	macs          map[string]int64
	offlineEvent  *time.Timer `json:"-"`
	isFirstBeat   bool
	sidx          uint16 `json:"-"`
	ridx          uint16 `json:"-"`
	a             int64
	z             int64
	mz            []byte
	output        []byte `response data for first login`
	stateLock     *sync.Mutex

	ws  *websocket.Conn
	adr string

	killchan chan *KillEvent
	syncchan chan int
	killer   *DevSession

	gws      *gw.Conn
	gwclient chan *[]byte

	client *net.TCPConn
}
type KillEvent struct {
	cmd    int
	killer *DevSession
}

func newDevTcpSession(ws *net.TCPConn) *DevSession {
	u := &DevSession{
		client:           ws,
		gwclient:      make(chan *[]byte, 10000),
		adr:           ws.RemoteAddr().String(),
		lastHeartbeat: time.Now(),
		isFirstBeat:   true,
		a:             time.Now().Unix(),
		devs:          make(map[int64]string),
		macs:          make(map[string]int64),
		stateLock:     &sync.Mutex{},
		killchan:      make(chan *KillEvent, 10),
		syncchan:      make(chan int, 10),
	}
	u.sid = fmt.Sprintf("%x", RandomByte8())
	return u
}
func newDevGwSession(ws *gw.Conn) *DevSession {
	u := &DevSession{
		gws:           ws,
		gwclient:      make(chan *[]byte, 1000),
		adr:           ws.RemoteAddr().String(),
		lastHeartbeat: time.Now(),
		isFirstBeat:   true,
		a:             time.Now().Unix(),
		devs:          make(map[int64]string),
		macs:          make(map[string]int64),
		stateLock:     &sync.Mutex{},
		killchan:      make(chan *KillEvent, 10),
		syncchan:      make(chan int, 10),
	}
	u.sid = fmt.Sprintf("%x", RandomByte8())
	return u
}

func newDevWsSession(ws *websocket.Conn) *DevSession {
	u := &DevSession{
		ws:            ws,
		adr:           ws.Request().RemoteAddr,
		gwclient:      make(chan *[]byte, 1000),
		lastHeartbeat: time.Now(),
		isFirstBeat:   true,
		a:             time.Now().Unix(),
		devs:          make(map[int64]string),
		macs:          make(map[string]int64),
		stateLock:     &sync.Mutex{},
		killchan:      make(chan *KillEvent, 10),
		syncchan:      make(chan int, 10),
	}
	u.sid = fmt.Sprintf("%x", RandomByte8())
	return u
}
func newDevSession(addr *net.UDPAddr) *DevSession {
	u := &DevSession{
		udpAdr:        addr,
		adr:           addr.String(),
		lastHeartbeat: time.Now(),
		isFirstBeat:   true,
		a:             time.Now().Unix(),
		devs:          make(map[int64]string),
		macs:          make(map[string]int64),
		stateLock:     &sync.Mutex{},
	}
	u.sid = fmt.Sprintf("%x", RandomByte8())
	return u
}
func (s *DevSession) getSid() []byte {
	sid, _ := hex.DecodeString(s.sid)
	return sid
}

// check pack number and other things in session here
func (s *DevSession) VerifyPack(packNum uint16) error {
	switch {
	case packNum > s.ridx:
	case packNum < s.ridx && packNum < 10 && s.ridx > uint16(65530):
	default:
		return fmt.Errorf("Wrong Package Sequence Number")
	}
	s.ridx = packNum
	return nil
}

func (s *DevSession) isBinded(id int64) bool {
	for _, v := range s.users {
		if v == id {
			return true
		}
	}
	//s.BindedUsers = GetDeviceUsers(s.DeviceId)
	return false
}

func (s *DevSession) CalcDestIds(toId int64) []int64 {
	if toId == 0 {
		return s.users
	} else {
		if !s.isBinded(toId) {
			users, owner, err := GetUsersByDev(s.devId)
			if err != nil {
				return nil
			}
			s.users = users
			s.owner = owner
			if owner == toId {
				return []int64{toId}
			}
			for _, uid := range users {
				if uid == toId {
					return []int64{toId}
				}
			}
			if glog.V(3) {
				glog.Errorf("dev [%d] unbind to usr [%d], current users: %v", s.devId, toId, s.users)
			}
			return nil
		}
		return []int64{toId}
	}
}

func (s *DevSession) Update(addr *net.UDPAddr) {
	if s == nil {
		return
	}
	if addr != nil && s.udpAdr.String() != addr.String() {
		s.udpAdr = addr
	}
	s.lastHeartbeat = time.Now()
	if s.offlineEvent != nil {
		s.offlineEvent.Reset(time.Duration(UdpTimeout) * time.Second)
		if glog.V(3) {
			glog.Infof("[HB]%v,%v,%v", s.sid, s.devId, s.udpAdr.String())
		}
	}
	return
}

func (s *DevSession) String() string {
	return fmt.Sprintf("[dev:%v %v %s %v]", s.devId, s.mac, s.sid, s.udpAdr.String())
}

func (s *DevSession) ToString() string {
	buf, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", s.mac) + string(buf)
}

func (s *DevSession) FromString(data string) error {
	return json.Unmarshal([]byte(data), s)
}

type DevSessionPool struct {
	Server *UdpServer
	Devlk  *sync.RWMutex
	Sesses map[int64]*DevSession
	Sids   map[string]*DevSession
	Macs   map[string]*DevSession
	MacSid map[string]string
}

func NewDevSessionPool() *DevSessionPool {
	return &DevSessionPool{
		Devlk:  new(sync.RWMutex),
		Sesses: make(map[int64]*DevSession),
		Sids:   make(map[string]*DevSession),
		Macs:   make(map[string]*DevSession),
		MacSid: make(map[string]string),
	}
}

func (this *DevSessionPool) add(o *DevSession) {
	this.Devlk.Lock()
	this.Sids[o.sid] = o
	this.Macs[o.mac] = o
	this.MacSid[o.mac] = o.sid
	this.Devlk.Unlock()
}
func (this *DevSessionPool) delSessionByMac(mac *string) {
	DevSessions.Devlk.Lock()
	delete(DevSessions.Macs, *mac)
	delete(DevSessions.Sids, DevSessions.MacSid[*mac])
	delete(DevSessions.MacSid, *mac)
	DevSessions.Devlk.Unlock()
}

func (this *DevSessionPool) PushCommonMsg(msgId uint16, did int64, msgBody []byte) error {
	msg := &msgs.Msg{}
	msg.FHOpcode = 2
	msg.FHMask = true
	msg.DHMsgId = msgId
	msg.FHDstId = did

	sess, _ := this.Sesses[did]
	if sess == nil {
		return fmt.Errorf("[udp:err] no session %v", did)
	}

	sess.sidx++
	msg.FHSequence = sess.sidx
	msg.Msg2Binary()
	UdpSrv.Send(sess.udpAdr, msg)
	return nil
}

//func (this *UdpSessionList) PushCommonMsg(msgId uint16, did int64, msgBody []byte) error {
//	msg := msgs.NewMsg(msgBody, nil)
//	msg.FrameHeader.Opcode = 2
//	msg.DataHeader.MsgId = msgId
//	msg.FrameHeader.DstId = did
//
//	sess, _ := this.Sesses[did]
//	if sess == nil {
//		return fmt.Errorf("[udp:err] no session %v", did)
//	}
//
//	sess.Sidx++
//	msg.FrameHeader.Sequence = sess.Sidx
//	msgBytes := msg.MarshalBytes()
//	this.Server.Send(sess.Addr, msgBytes)
//	return nil
//}

func (this *DevSessionPool) PushMsg(did int64, msg []byte) {
	var (
	//		err error
	)
	sess, _ := this.Sesses[did]
	if sess == nil {
		glog.Errorf("[PUSH] FAIL dev:%d|%d|%x", did, len(msg), msg)
		return
	}

	m := new(msgs.Msg)
	m.Final = msg
	m.Binary2Msg()
	if m.FHOpcode == 5 {
		if did == sess.devId {
			m.FHDstMac = sess.mac
		} else {
			m.FHDstMac = sess.devs[m.FHDstId]
		}
		if strings.Trim(m.FHDstMac, " ") == "" {
			glog.Errorf("err: %d's mac:%s", did, m.FHDstMac)
			return
		}
		dstMac, _ := hex.DecodeString(m.FHDstMac)
		m.FHDstId = int64(binary.LittleEndian.Uint64(dstMac))
		if sess.mz == nil {
			glog.Errorf("err: mz is nil, %v, mz:%x", sess, sess.mz)
			return
		}
		m.MZ = sess.mz
		m.DHKeyLevel = 3
		m.FHMask = true
		m.Msg2Binary()
		msg = m.Final
	}
	switch SrvType {
	case CometUdp:
		data := &Datagram{data: &msg, adr: sess.udpAdr, aim: did, mac: sess.mac}
		UdpSrv.Queue <- data
		glog.Infof("[DEV_UDP] ->%d,%x", did, msg)
	case CometDw:
		sess.gwclient <- &msg
		glog.Infof("[DEV_WS] ->%d,%x", did, msg)
	}

}

func (this *DevSessionPool) UpdateIds(deviceId int64, userId int64, bindType bool) {
	sess, _ := this.Sesses[deviceId]
	if sess == nil {
		glog.Errorf("[udp:bind] no udp session %v when updating mapping dev<->usr", deviceId)
		return
	}
	if bindType {
		// 绑定
		sess.users = append(sess.users, userId)
		if glog.V(3) {
			glog.Infof("[udp:bind] dev:%d binded usr:%d", deviceId, userId)
		}
		GSentinelMgr.NotifyBindedIdChanged(deviceId, []int64{userId}, nil)

	} else {
		// 解绑
		for k, v := range sess.users {
			if v != userId {
				continue
			}
			lastIndex := len(sess.users) - 1
			sess.users[k] = sess.users[lastIndex]
			sess.users = sess.users[:lastIndex]
			if glog.V(3) {
				glog.Infof("[udp:unbind] dev:%d unbinded usr:%d", deviceId, userId)
			}
			break
		}
		GSentinelMgr.NotifyBindedIdChanged(deviceId, nil, []int64{userId})
	}
}
