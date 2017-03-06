package srv

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	//	"net"
	"github.com/golang/glog"
	aes "octopus/common/aes"
	"octopus/msgs"
	"octopus/websocket"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	killedByOtherDevice  = "Another device login %d"
	wrongLoginParams     = "Wrong login params %s"
	wrongLoginType       = "Wrong login type %s"
	wrongLoginDevice     = "Wrong login deviceId %s"
	wrongLoginTimestamp  = "Wrong login timestamp %s"
	userLoginTimeout     = "User %d login timeout"
	userReconnectTimeout = "Reconnect timeout %d"
	wrongMd5Check        = "User %d has wrong md5"
	wrongLoginTimeout    = "Wrong login %d timeout %s"

	LOGIN_PARAM_COUNT = 4
	READ_TIMEOUT      = 120
	TIME_OUT          = 150
	EXPIRE_TIME       = int64(1 * 60) // 1 mins

	kLoginKey = "BlackCrystalWb14527" // 和http服务器约定好的私有盐

	// 用户id的间隔，这个间隔内可用的取值数量，就是该用户可同时登录的手机数量

	kDstIdOffset = 8
	kDstIdLen    = 8
	kDstIdEnd    = kDstIdOffset + kDstIdLen
)

var (
	LOGIN_PARAM_ERROR = errors.New("Login params parse error!")

	AckLoginOK          = []byte{byte(0)} // 登陆成功
	AckWrongParams      = []byte{byte(1)} // 错误的登陆参数
	AckCheckFail        = []byte{byte(2)} // 登录验证失败
	AckNotifyMsgbusFail = []byte{byte(3)} // 通知MSGBUS用户登录MSGBUS失败
	//	AckWrongLoginTimestamp = []byte{byte(4)}  // 登陆时间戳解析错误
	//	AckLoginTimeout        = []byte{byte(5)}  // 登陆超时
	//	AckWrongMD5            = []byte{byte(6)}  // 错误的md5
	//	AckOtherglogoned       = []byte{byte(7)}  // 您已在别处登陆
	//	AckWrongLoginTimeout   = []byte{byte(8)}  // 超时解析错误
	AckServerError      = []byte{9}  // 服务器错误
	AckModifiedPasswd   = []byte{10} // 密码已修改
	AckSecondLogin      = []byte{11}
	AckForceUserOffline = []byte{12}
	urlLock             sync.Mutex
)

func verifyLoginParams(req string) (id int64, timestamp, timeout uint64, md5Str string, MobileID string, err error) {
	args := strings.Split(req, "|")
	// skip the 0
	id, err = strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return
	}
	if len(args[1]) == 0 {
		err = errors.New("login needs timestamp")
		return
	}
	timestamp, err = strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return
	}
	if len(args[2]) == 0 {
		err = errors.New("login needs timeout")
		return
	}
	timeout, err = strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return
	}
	md5Str = args[3]
	if len(args) >= 5 {
		MobileID = args[4]
	}
	return
}

func isAuth(id int64, timestamp uint64, timeout uint64, md5Str string) error {
	if timestamp > 0 && int64(uint64(time.Now().Unix())-timestamp) >= EXPIRE_TIME {
		return fmt.Errorf("login timeout, %d - %d >= %d", uint64(time.Now().Unix()), timestamp, EXPIRE_TIME)
		// return false
	}
	// check hmac is equal
	md5Buf := md5.Sum([]byte(fmt.Sprintf("%d|%d|%d|%s", id, timestamp, timeout, kLoginKey)))
	md5New := base64.StdEncoding.EncodeToString(md5Buf[0:])
	if md5New != md5Str {
		if glog.V(1) {
			glog.Warningf("auth not equal: %s != %s", md5New, md5Str)
		}
		return errors.New("login parameter is not verified")
	}
	return nil
}

func setReadTimeout(conn *websocket.Conn, delaySec int) error {
	return conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(delaySec)))
}

func mwh(ws *websocket.Conn) {
	glog.Infof("LIFE %v", ws.Request().RemoteAddr)
	s := new(UsrSession)
	s.ws = ws
	s.adr = ws.Request().RemoteAddr
	var (
		err   error
		reply []byte
	)
	if err = websocket.Message.Receive(ws, &reply); err != nil {
		glog.Errorf("[ws:err] %v websocket.Message.Receive() error(%v)", ws.Request().RemoteAddr, err)
		ws.Close()
		return
	}

	/**new begin*/
	var (
		ctn       string
		ctnary    []string
		input     map[string]string
		httpresp  map[string]interface{}
		m         *msgs.Msg
		tmpcipher []byte
	)
	m = new(msgs.Msg)
	m.Final = reply
	err = m.Binary2Msg()
	if err != nil {
		goto old
	}
	if glog.V(5) {
		glog.Infof("ws new:%v:%s", s.adr, m)
	}
	s.uid = m.FHSrcId
	ctn = string(m.Text)
	ctnary = strings.Split(ctn, "|")
	if len(ctnary) != 2 {
		glog.Errorf("[ws:err] bad ctn. %v,%d,%s", s.adr, s.uid, ctn)
		ws.Close()
		return
	}
	s.token = ctnary[0]
	s.mobileID = ctnary[1]
	glog.Infof("[ws:auth] %d,%s,%s", s.uid, s.token, s.mobileID)
	//got USR AUTHCODE
	input = make(map[string]string)
	input["userId"] = fmt.Sprintf("%d", m.FHSrcId)
	input["token"] = s.token

	_, httpresp, err = get(AUTHURL, input)
	if glog.V(5) {
		glog.Infof("[auth] %v,%v", httpresp, err)
	}
	if err == nil {
		if v, ok := httpresp["data"]; ok {
			if data, ok := v.(map[string]interface{}); ok {
				if v, ok := data["status"]; ok {
					if n, ok := v.(float64); ok {
						status := int32(n)
						if status == 0 {
							if v, ok = data["key"]; ok {
								if auth, ok := v.(string); ok {
									authcode, err := hex.DecodeString(auth)
									if err != nil {
										glog.Errorf("[auth:err] authcode:%d,%v", auth, err)
										ws.Close()
										return
									}

									tmpcipher, s.mz, err = gotTmpkey(authcode)
									if err != nil {
										glog.Errorf("[auth:err] %v", err)
										ws.Close()
										return
									}

								}
							}
						} else {
							glog.Errorf("[auth:err] status:%d", status)
							ws.Close()
							return
						}
					}
				}
			}
		} else {
			glog.Errorf("[auth:err] no data")
			ws.Close()
			return
		}
	} else {
		glog.Errorf("[auth:err] %v", err)
		ws.Close()
		return
	}
	m.FHDstId = m.FHSrcId
	m.FHSrcId = 0
	//	if tmpcipher == nil || len(tmpcipher) == 0 {
	//		m.Text, s.mz, _ = gotTmpkey([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	//	} else {
	m.Text = tmpcipher
	//	}
	m.Msg2Binary()
	_, err = s.ws.Write(m.Final)
	if glog.V(5) {
		glog.Infof("[ws:ret] %x %v", m.Final, err)
	}

	goto common
	/**new end*/

	//old login logic
old:
	if glog.V(5) {
		glog.Infof("[ws:old] %v:%s", s.adr, string(reply))
	}
	//	parse login params
	s.uid, s.timestamp, s.timeout, s.sequence, s.mobileID, err = verifyLoginParams(string(reply))
	//	if _, ok := gSessionList.onlined[id]; ok {
	//		glog.Errorf("[ws:err] %v has been logined. [%s] params (%s) error (%v)\n", id, clientAdr, string(reply), loginErr)
	//		websocket.Message.Send(ws, AckSecondLogin)
	//		ws.Close()
	//		return
	//	}

	if err != nil {
		glog.Errorf("[ws:%v] %s (%v)", s, string(reply), err)
		websocket.Message.Send(ws, AckWrongParams)
		ws.Close()
		return
	}
	// check login
	if err = isAuth(s.uid, s.timestamp, s.timeout, s.sequence); err != nil {
		glog.Errorf("[ws:%v] auth failed:\"%s\", error: %v", s, string(reply), err)
		websocket.Message.Send(ws, AckCheckFail)
		ws.Close()
		return
	}

	//common login code
common:
	ForceUserOffline(s.uid, s.mobileID)

	s.devs, _ = GetDevByUsr(s.uid)

	_, err = Login(s.uid, *SentinelAdr, s.mobileID)
	if err != nil {
		_, err := ws.Write(AckNotifyMsgbusFail)
		glog.Errorf("[ws:%v] AckNotifyMsgbusFail %v", s, err)
		ws.Close()
		return
	}

	s.a = time.Now().Unix()
	UsrSessions.AddSession(s)
	defer logout(s, err)
	// 回应成功登陆
	err = websocket.Message.Send(ws, AckLoginOK)
	if err != nil {
		glog.Errorf("[ws:%v] %v", s, err)
		return
	}
	for offlineMsg := popOfflineMsg(s.uid); offlineMsg != nil; offlineMsg = popOfflineMsg(s.uid) {
		err = websocket.Message.Send(ws, offlineMsg)
		glog.Infof("[ws:pop]%s %x", s, offlineMsg)
		if err != nil { //发送失败把消息再塞回离线消息队列
			pushOfflineMsg(s.uid, offlineMsg)
			return
		}
	}
	if s.timeout <= 0 {
		s.timeout = TIME_OUT
	}
	ws.ReadTimeout = time.Duration(3*s.timeout) * time.Second
	if glog.V(5) {
		glog.Infof("[ws:logined]  %v", s)
	}
	statIncConnOnline()
	statIncConnTotal()
	defer statDecConnOnline()
	for {
		if err = websocket.Message.Receive(ws, &reply); err != nil {
			glog.Errorf("[ws:%v] %v", s, err)
			break
		}
		if len(reply) > 1024 {
			glog.Errorf("TOO LONG ERR, THE LENGTH OF REQ: %d", len(reply))
			continue
		}
		msg := reply
		if len(msg) < kDstIdEnd {
			if string(msg) == "p" {
				if glog.V(5) {
					glog.Infof("[ws:pong] %s", s)
				}
				websocket.Message.Send(ws, "P")
				continue
			} else {
				glog.Errorf("[ws] %s", s)
				break
			}
		}
		statIncUpStreamIn()

		m := &msgs.Msg{}
		m.MZ = s.mz
		m.Final = msg
		err = m.Binary2Msg()
		if glog.V(5) {
			glog.Infof("[ws:req]:%s,%s,%v ", s, m, err)
		}
		r := &msgs.Msg{}
		r.DHAck = true
		r.DHRead = true
		r.FHFin = true
		r.FHMask = m.FHMask
		r.DHMsgId = m.DHMsgId
		r.FHOpcode = m.FHOpcode
		r.DHDataSeq = m.DHDataSeq
		r.DHKeyLevel = m.DHKeyLevel
		r.DHEncryptType = m.DHEncryptType
		r.FHSrcId = 0
		r.FHDstId = m.FHSrcId
		r.FHTime = uint32(time.Now().Unix())
		if m.FHOpcode == 2 {
			switch m.DHMsgId {
			case 0xc7:
				dst := int64(binary.LittleEndian.Uint64(m.Text[0:8]))
				if glog.V(5) {
					glog.Infof("%s %d -> %d", s, m.FHSrcId, dst)
				}
				GSentinelMgr.Forward([]int64{dst}, msg)

				txt := make([]byte, 12)
				binary.LittleEndian.PutUint32(txt[:4], 0)
				binary.LittleEndian.PutUint64(txt[4:12], uint64(m.FHDstId))
				r.Text = txt
				r.Msg2Binary()
				if glog.V(5) {
					glog.Infof("%s sys -> %d %s", s, m.FHSrcId, r.String())
				}
				websocket.Message.Send(ws, r.Final)
			}
			continue
		} else if m.FHOpcode == 5 {
			if m.MZ == nil {
				glog.Errorf("[ws:err] %s err: opcode = 6 and MZ = nil", s)
				break
			}
			if err != nil {
				glog.Errorf("[ws:err] %s err: opcode = 6 and bad req", s)
				break
			}

			m.DHKeyLevel = 0
			m.Msg2Binary()

			dest := m.FHDstId
			destIds := s.calcDestIds(dest)
			if len(destIds) > 0 {
				statIncUpStreamOut()
				GSentinelMgr.Forward(destIds, m.Final)
			}
			if glog.V(5) {
				glog.Infof("[ws:f] %s %d -> (expected %d|actual%v)", s, s.uid, dest, destIds)
			}
			continue
		} else if m.FHOpcode == 3 {
			// 根据手机与嵌入式协议，提取消息中的目标id
			dest := m.FHDstId

			destIds := s.calcDestIds(dest)
			if len(destIds) > 0 {
				statIncUpStreamOut()
				GSentinelMgr.Forward(destIds, msg)
			}
			if glog.V(5) {
				glog.Infof("[ws:f]%s %d -> (expected %d|actual%v)", s, s.uid, dest, destIds)
			}
		} else {
			if glog.V(5) {
				glog.Infof("[ws:err] opcode = %d,%s", m.FHOpcode, m)
			}
		}
	}
}
func logout(s *UsrSession, err error) {
	if s.uid > 0 {
		statDecConnOnline()
		Logout(s.uid, *SentinelAdr)
		UsrSessions.RemoveSession(s)
	}
	if glog.V(3) {
		s.z = time.Now().Unix()
		a := time.Unix(s.a, 0).Format("2006-01-02 15:04:05")
		b := time.Unix(s.z, 0).Format("2006-01-02 15:04:05")
		glog.Infof("OVER %v lasted for [%d]s. from [%s] to [%s],%v", s, s.z-s.a, a, b, err)
	}
}

func gotTmpkey(authcode []byte) ([]byte, []byte, error) {
	var c []byte
	tmpkey := RandomByte16()[:]
	c = append(c, tmpkey...)
	c = append(c, authcode[:]...)
	mz := md5.Sum(c)
	tmpcipher, err := aes.EncryptNp(tmpkey, authcode)
	if err != nil {
		glog.Errorf("[ws:err] %v", err)
		return nil, nil, err
	}
	return tmpcipher, mz[:], nil
}
