package srv

import (
	"octopus/msgs"
	"cloud-base/websocket"
	"time"

	"github.com/golang/glog"
)

func dwh(ws *websocket.Conn) {
	glog.Infof("DWH %s", ws.Request().RemoteAddr)
	var (
		t    = new(DevRequest)
		ms   = newDevWsSession(ws)
		err  error
		resp *msgs.Msg
		req  []byte
		busi string
	)
	defer devLogout(ms, err)

start:
	if err = websocket.Message.Receive(ws, &req); err != nil {
		glog.Errorf("[dw:err] %v error(%v)", ws.Request().RemoteAddr, err)
		ws.Close()
		return
	}
	glog.Infof("RECV:%s,%x", ms.adr, req)
	m := &msgs.Msg{}
	m.Final = req
	m.Binary2Msg()
	resp = new(msgs.Msg)
	resp.DHMsgId = m.DHMsgId
	resp.FHMask = m.FHMask
	resp.DHAck = true
	resp.DHRead = true
	resp.DHKeyLevel = m.DHKeyLevel
	resp.DHEncryptType = m.DHEncryptType
	resp.FHOpcode = 2
	resp.DHDataSeq = m.DHDataSeq
	resp.FHSequence = m.FHSequence
	resp.FHTime = uint32(time.Now().Unix())
	resp.FHSrcId = 0x00
	resp.FHDstId = m.FHSrcId

	ms.mac = m.FHSrcMac

	switch m.DHMsgId {
	case CmdLogin:
		var state int32
		resp.Text, err, state = onLogin(ms, m)
		if err != nil {
			glog.Errorf("[dw:err] in login %v", err)
			goto start
		} else {
			resp.Msg2Binary()
			err = websocket.Message.Send(ms.ws, resp.Final)
			if state != 0 {
				goto start
			}
		}
	case CmdRegister:
		resp.Text, err = onRegister(ms, m)
		if err != nil {
			glog.Errorf("[dw:err] in reg %v", err)
		} else {
			resp.Msg2Binary()
			_, err = ms.ws.Write(resp.Final)
		}
		goto start
	default:
		glog.Errorf("[dw:err] bad first request %s", m)
		goto start
	}
	go func() {
		for tip := range ms.gwclient {
			err := websocket.Message.Send(ms.ws, tip)
			if err != nil {
				glog.Infof("%s %v", ms, err)
			}
		}
	}()
	ws.ReadTimeout = time.Duration(120) * time.Second
	for {
		if err = websocket.Message.Receive(ws, &req); err != nil {
			glog.Errorf("[dw:err]%s %v", ms, err)
			return
		}
		ms.lastHeartbeat = time.Now()
		m := &msgs.Msg{}
		m.Final = req
		m.Binary2Msg()
		t.Msg = m
		if glog.V(5) {
			glog.Infof("RECV:%d %x", ms.devId, req)
		}
		statIncUpStreamIn()

		switch m.FHOpcode {
		case 0x02:
			_, _, err, busi, resp = opcode02(t, ms)
			if err == nil {
				ms.gwclient <- &resp.Final
				glog.Infof("%s %s %v %d", busi, resp, err)
			}
		case 0x03:
			_, _, err = opcode03(m, ms)
		case 0x05:
			_, _, err = opcode05(m, ms)
		default:
			glog.Errorf("[dw:err]!!opcode=%d,%s", m.FHOpcode, ms)
		}
		if err != nil {
			glog.Errorf("err!!%v,%s", err, m)
		}

	}
}
