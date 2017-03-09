package srv

import (
	"sync"

	"fmt"
	"octopus/msgs"
	"cloud-base/websocket"

	"github.com/golang/glog"
	//"strings"
	//"strconv"
)

var (
	BlockSize int64 = 128
	MapSize   int64 = 1024
)

type UsrSessionPool struct {
	Sesses    map[int64]*UsrSession
	onlinedMu *sync.Mutex
}
type UsrSession struct {
	timeout   uint64
	timestamp uint64
	mobileID  string
	sequence  string
	uid       int64
	devs      []int64 // Uid为手机:包含所有已绑定的板子id;Uid为板子时，包含所有已绑定的用户id
	ws        *websocket.Conn
	adr       string
	a         int64
	z         int64
	token     string
	mz        []byte
	sendLock  sync.Mutex
}

func NewMobileSession() *UsrSession {
	s := &UsrSession{sendLock: sync.Mutex{}}
	return s
}
func (this *UsrSession) Close() {
	this.ws.Close()
}
func (this *UsrSession) String() string {
	return fmt.Sprintf("[%v %v %v %v %v]", this.uid, this.mobileID, this.adr, this.sequence, this.timestamp)
}
func (this *UsrSession) isBinded(id int64) bool {
	for _, v := range this.devs {
		if v == id {
			return true
		}
	}
	return false
}

func (this *UsrSession) calcDestIds(toId int64) []int64 {
	if toId == 0 {
		return this.devs
	} else {
		if !this.isBinded(toId) {
			devs, err := GetDevByUsr(this.uid)
			if err != nil {
				return nil
			}
			this.devs = devs
			for _, dev := range devs {
				if dev == toId {
					return []int64{toId}
				}
			}
			glog.Errorf("[msg] usr [%d] unbind dev [%d], valid ids: %v", this.uid, toId, this.devs)
			return nil
		}
		return []int64{toId}
	}
	return nil
}

func InitSessionList() *UsrSessionPool {
	sl := &UsrSessionPool{
		Sesses:    make(map[int64]*UsrSession, MapSize),
		onlinedMu: new(sync.Mutex),
	}
	return sl
}

func (this *UsrSessionPool) AddSession(s *UsrSession) {
	this.onlinedMu.Lock()
	defer this.onlinedMu.Unlock()
	if anotherSes, ok := this.Sesses[s.uid]; ok {
		anotherSes.ws.Close()
		if glog.V(3) {
			glog.Infof("[%s] indead of [%s]", s, anotherSes)
		}
		anotherSes.uid = -1 //为了在调用RemoveSession时不把后登录的删掉
	}
	if glog.V(3) {
		glog.Infof("NewSess %s", s)
	}
	this.Sesses[s.uid] = s
	return
}

func (this *UsrSessionPool) RemoveSession(s *UsrSession) {
	this.onlinedMu.Lock()
	defer this.onlinedMu.Unlock()
	if s.ws != nil {
		s.Close()
	}
	delete(this.Sesses, s.uid)
}

func (this *UsrSessionPool) UpdateIds(deviceId int64, userId int64, bindType bool) {
	this.onlinedMu.Lock()
	defer this.onlinedMu.Unlock()
	if s, ok := this.Sesses[userId]; ok {
		if !ok {
			if glog.V(3) {
				glog.Infof("[WS UPDATE DEVLIST] no session of %d, fail to bind/unbind device %d", userId, deviceId)
			}
			return
		}
		if bindType {
			// 绑定
			s.devs = append(s.devs, deviceId)
			if glog.V(3) {
				glog.Infof("[WS UPDATE DEVLIST] usr:%d has binded dev:%d", userId, deviceId)
			}
		} else {
			// 解绑
			for k, v := range s.devs {
				if v != deviceId {
					continue
				}
				lastIndex := len(s.devs) - 1
				s.devs[k] = s.devs[lastIndex]
				s.devs = s.devs[:lastIndex]
				if glog.V(3) {
					glog.Infof("[WS UPDATE DEVLIST] usr:%d has unbinded dev:%d", userId, deviceId)
				}
				break
			}
		}
	}
}

func (this *UsrSessionPool) PushCommonMsg(msgid uint16, dstId int64, msgBody []byte) {
	var err error
	if glog.V(3) {
		glog.Infof("[SEND COMMON MSG TO USR] Received=msgid:%v,usr:%v,len(msgBody):%v,msgBody:%v", msgid, dstId, len(msgBody), msgBody)
	}
	s, ok := this.Sesses[dstId]
	if !ok {
		if glog.V(3) {
			glog.Infof("[SEND COMMON MSG TO USR] MSG can't send, Destination is not in this WSCOMET,ADDITIONAL MSG msgid:%v,usr:%v,len(msgBody):%v,msgBody:%v", msgid, dstId, len(msgBody), msgBody)
		}
		return
	}
	msg := &msgs.Msg{}
	msg.Text = msgBody
	msg.FHOpcode = 2
	msg.FHMask = true
	msg.DHMsgId = msgid
	msg.FHDstId = dstId
	msg.Msg2Binary()
	msgBytes := msg.Final

	if glog.V(3) {
		glog.Infof("[SEND COMMON MSG TO USR] Final Msg |len:%v| |%v|", len(msgBytes), msgBytes)
	}
	_, err = s.ws.Write(msgBytes)
	if err != nil {
		if glog.V(3) {
			glog.Infof("[SEND COMMON MSG TO USR] Failed |%v|", err)
		}
		return
	}
	if glog.V(3) {
		glog.Infof("[SEND COMMON MSG TO USR] DONE %v->%v,MsgID:%v,ctn:%v", s.adr, dstId, msgid, msgBytes)
	}
}

func (this *UsrSessionPool) OffliningUsr(uid int64) {
	if glog.V(3) {
		glog.Infof("[OFFLINE USR] %v", uid)
	}
	var err error
	sess, ok := this.Sesses[uid]
	if !ok {
		if glog.V(3) {
			glog.Infof("[OFFLINE USR] no usr:%v is in this WSCOMET", uid)
		}
		return
	}
	body := msgs.MsgStatus{}
	body.Type = msgs.MSTKickOff

	kickMsg := &msgs.Msg{}
	kickMsg.FHMask = true
	kickMsg.FHOpcode = 2
	kickMsg.DHMsgId = msgs.MIDStatus

	body.Id = uid
	kickMsg.FHDstId = uid
	kickMsg.Text, _ = body.Marshal()
	kickMsg.Msg2Binary()
	err = websocket.Message.Send(sess.ws, kickMsg.Final)
	if err != nil {
		glog.Warningf("[OFFLINE USR] usr:%d,%v", sess.uid, err)
	}
	err = sess.ws.Close()
	if err != nil {
		glog.Warningf("[OFFLINE USR] usr:%d, error: %v", sess.uid, err)
	}
	if glog.V(3) {
		glog.Infof("[OFFLINE USR] Done. usr:%v", uid)
	}
}
func (this *UsrSessionPool) PushMsg(uid int64, data []byte) {
	var err error
	if s, ok := this.Sesses[uid]; ok {
		if s.mz != nil {
			m := new(msgs.Msg)
			m.Final = data
			m.Binary2Msg()
			if m.FHOpcode == 5 {
				m.MZ = s.mz
				m.FHMask = true
				m.DHKeyLevel = 3
				m.Msg2Binary()
				data = m.Final
			}
		}
		_, err = s.ws.Write(data)

		if err != nil {
			glog.Errorf("[WSPUSH] FAILED! usr:%d,%v,%x,%v", s.uid, len(data), data, err)
		} else {
			statIncDownStreamOut()
			if glog.V(3) {
				glog.Infof("[WSPUSH] DONE! usr:%d, data: (len %d)%x", s.uid, len(data), data)
			}
		}
	} else {
		if glog.V(3) {
			glog.Infof("[WSPUSH] FAIL! usr:%d|%d|%x", uid, len(data), data)
		}
	}
}
