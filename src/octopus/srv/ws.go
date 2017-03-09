package srv

import (
	"encoding/binary"
	"fmt"
	"github.com/golang/glog"
	gw "github.com/gorilla/websocket"
	"io"
	"net"
	"net/http"
	"octopus/msgs"
	"cloud-base/websocket"
	"time"
)

var (
	FRAMEFLAG = []byte{0xfa, 0xc3, 0xc6, 0xc9, 0x05, 0x3c, 0x39, 0x36}
	pongWait  = 120 * time.Second
)

func reader(ws *gw.Conn) {
	defer ws.Close()
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		t, r, err := ws.ReadMessage()
		if err != nil {
			break
		}
		glog.Infof("recv:%v %x", t, r)

	}
}
func writer(ms *DevSession) {
	go func() {
		for p := range ms.gwclient {
			if err := ms.gws.WriteMessage(gw.BinaryMessage, *p); err != nil {
				glog.Info(err)
				return
			}
		}
	}()
}

type GwServer struct {
}

func (g GwServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	glog.Infof("GW: ", g)
	upgrader := gw.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		if _, ok := err.(gw.HandshakeError); !ok {
			glog.Info(err)
		}
		return
	}
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	dgw(ws)
	ws.Close()
}

func dgw(ws *gw.Conn) {
	glog.Infof("DWH %s", ws.RemoteAddr().String())
	var (
		t    = new(DevRequest)
		ms   = newDevGwSession(ws)
		err  error
		resp *msgs.Msg
		busi string
	)
	defer devLogout(ms, err)
	go writer(ms)
start:
	_, req, err := ws.ReadMessage()
	if err != nil {
		glog.Errorf("[dw:err] %s error(%v)", ms.adr, err)
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
			ms.gwclient <- &resp.Final
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
			ms.gwclient <- &resp.Final
		}
		goto start
	default:
		glog.Errorf("[dw:err] bad first request %s", m)
		goto start
	}

	for {
		_, req, err = ws.ReadMessage()
		if err != nil {
			glog.Errorf("[dw:err] %s error(%v)", ms.adr, err)
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

func StartGW() {
	h := new(GwServer)
	launchSrv(h)
}
func LaunchWS() {
	var handler func(*websocket.Conn)

	switch SrvType {
	case CometWs:
		handler = mwh
	case CometDw:
		handler = dwh
	}
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

	launchSrv(wsHandler)
}

func launchSrv(h http.Handler) {
	httpServeMux := http.NewServeMux()
	httpServeMux.Handle("/ws", h)
	httpServeMux.Handle("/", h)
	server := &http.Server{
		Addr:        ME.String(),
		Handler:     httpServeMux,
		ReadTimeout: READ_TIMEOUT * time.Second,
	}

	glog.Infof("I start at %s,%d", ME.String(), SrvType)

	err := server.ListenAndServe()
	if err != nil {
		glog.Errorf("[%s] %v", ME.String(), err)
		panic(err)
	}
}

func LaunchDevSrv() {
	server, err := net.ListenTCP("tcp", ME)
	glog.Error(err)
	for {
		client, er := server.AcceptTCP()
		d := time.Now().Add(120 * time.Second)
		client.SetReadDeadline(d)
		if err != nil {
			glog.Error(er)
			continue
		}
		go busiDeal(client)
	}
}

func tcpWriter(ms *DevSession) {
	go func() {
		for p := range ms.gwclient {
			var final []byte
			final = append(final, FRAMEFLAG...)
			dl := len(*p)
			bdl := make([]byte, 2)
			binary.LittleEndian.PutUint16(bdl, uint16(dl))
			final = append(final, bdl...)
			final = append(final, *p...)
			if n, err := ms.client.Write(final); err != nil {
				glog.Info(n, err)
				return
			}
			glog.Infof("DONE! %x->%d", final, ms.devId)
		}
	}()
}
func busiDeal(ws *net.TCPConn) {
	glog.Infof("DWH %s", ws.RemoteAddr().String())
	var (
		t    = new(DevRequest)
		ms   = newDevTcpSession(ws)
		err  error
		resp *msgs.Msg
		busi string
	)
	defer devLogout(ms, err)
	go tcpWriter(ms)
start:
	req, err := readFrame(ws)
	d := time.Now().Add(120 * time.Second)
	ws.SetReadDeadline(d)
	if err != nil {
		glog.Errorf("[dw:err] %s error(%v)", ms.adr, err)
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
			ms.gwclient <- &resp.Final
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
			ms.gwclient <- &resp.Final
		}
		goto start
	default:
		glog.Errorf("[dw:err] bad first request %s", m)
		goto start
	}

	for {
		req, err = readFrame(ws)
		if err != nil {
			glog.Errorf("[dw:err] %s error(%v)", ms.adr, err)
			return
		}
		d := time.Now().Add(120 * time.Second)
		ws.SetReadDeadline(d)
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
func readFrame(c *net.TCPConn) (frame []byte, err error) {
	head := make([]byte, 8)
	//	n, err := c.Read(head)
	n, err := io.ReadFull(c, head)
	if err != nil {
		glog.Error(err)
		return
	}
	glog.Infof("RECV first %x %d %v", head, n, err)
	var erbt []byte
loop:
	i := 0
	for ; i < 8; i++ {
		if FRAMEFLAG[i] != head[i] {
			erbt = append(erbt, head[i])
			fmt.Printf("%x", head[i])
			one := make([]byte, i+1)
			_, err = c.Read(one)
			//			_, err = io.ReadFull(c, one)
			head = head[i+1:]

			if err != nil {
				return
			}
			head = append(head, one...)
			goto loop
		}
	}
	fmt.Println("")
	if len(erbt) > 0 {
		glog.Infof("erbt:%x", erbt)
	}
	bdl := make([]byte, 2)
	//	_, err = c.Read(bdl)
	_, err = io.ReadFull(c, bdl)
	glog.Infof("len: %x", bdl)
	if err != nil {
		glog.Error(err)
		return
	}
	dl := binary.LittleEndian.Uint16(bdl)
	frame = make([]byte, dl)
	//	n, err = c.Read(frame)
	n, err = io.ReadFull(c, frame)
	glog.Infof("RECV frame %x %d %v", frame, n, err)
	return
}
