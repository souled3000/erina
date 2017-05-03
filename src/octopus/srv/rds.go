package srv

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"octopus/common"
	"octopus/msgs"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/golang/glog"
)

const (
	HostPtn              = "Host:%s"
	HostPrefix           = "Host:"
	OnOff                = "OnOff"
	NewDevSessChannel    = "NewDevSessChannel"
	SubDeviceUsersKey    = "PubDeviceUsers"
	SubModifiedPasswdKey = "PubModifiedPasswdUser"
	ZiDevOnlineChannel   = "ZiDevOnlineChannel"
	RedisDeviceUsers     = "device:owner"
	RedisUserDevices     = "u:%v:devices"
	DEVADRKEY            = "device:adr"
)
const (
	_scriptOnline = `
local count = redis.call('hincrby', KEYS[1], KEYS[2], 1)
if count == 1 then
	redis.call('publish',  'OnOff', KEYS[2].."|"..KEYS[3].."|1")
end
if tonumber(KEYS[2]) < 0 then
	redis.call('hset','d:srv',KEYS[2],KEYS[3])
end
if tonumber(KEYS[2]) > 0 then
	redis.call('hset','m:srv',KEYS[2],KEYS[3])
	redis.call('hset','m:id',KEYS[2],KEYS[4])
end
return count
`

	_scriptOffline = `
redis.call('hdel', KEYS[1], KEYS[2])
redis.call('publish', 'OnOff', KEYS[2].."|"..KEYS[3].."|0")
if tonumber(KEYS[2]) < 0 then
	redis.call('hdel','d:srv',KEYS[2])
end
if tonumber(KEYS[2]) > 0 then
	redis.call('hdel','m:srv',KEYS[2])
	redis.call('hdel','m:id',KEYS[2])
end
return true
`
)

var (
	ScriptOnline  *redis.Script
	ScriptOffline *redis.Script
	Redix         *redis.Pool
)

func newPool(server, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle: 5,
		// MaxActive:   100,
		//IdleTimeout: 10 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			if err != nil {
				glog.Info(err)
			}
			return err
		},
	}
}

func ClearRedis(ip string) error {
	r := Redix.Get()
	defer r.Close()
	err := r.Send("del", fmt.Sprintf("Host:%s", ip))
	if err != nil {
		return err
	}
	err = r.Send("publish", OnOff, fmt.Sprintf("0|%s|0", ip))
	if err != nil {
		return err
	}
	err = r.Flush()
	if err != nil {
		return err
	}
	_, err = r.Receive()
	if err != nil {
		return err
	}
	_, err = r.Receive()
	if err != nil {
		return err
	}
	return nil
}
func InitRedix(addr string) {
	Redix = newPool(addr, "")
	r := Redix.Get()
	_, err := r.Do("PING")
	if err != nil {
		glog.Fatalf("Connect Redis [%s] failed [%s]", addr, err.Error())
	}
	defer r.Close()
	glog.Infof("RedisPool Init OK")

	//	ScriptSelectMobileId = redis.NewScript(1, _scriptSelectMobileId)
	//	ScriptSelectMobileId.Load(r)
	ScriptOnline = redis.NewScript(4, _scriptOnline)
	ScriptOnline.Load(r)
	ScriptOffline = redis.NewScript(3, _scriptOffline)
	ScriptOffline.Load(r)
	InitUsr2Sentinel()
	Sub()
}
func Sub() {
	go func() {
		for {
			r := Redix.Get()
			psc := redis.PubSubConn{Conn: r}
			switch {
			case SrvType == CometWs:
				psc.Subscribe(OnOff, SubDeviceUsersKey, SubModifiedPasswdKey)
			case SrvType == CometUdp || SrvType == CometDw:
				psc.Subscribe(OnOff, SubDeviceUsersKey, ZiDevOnlineChannel, NewDevSessChannel)
			}
		a:
			for {
				data := psc.Receive()
				switch n := data.(type) {
				case redis.Message:
					switch n.Channel {
					case OnOff:
						handleOnOff(n.Data)
					case SubModifiedPasswdKey:
						HandleModifiedPasswd(n.Data)
					case SubDeviceUsersKey:
						HandleDeviceUsers(n.Data)
					case NewDevSessChannel:
						func(m []byte) {
							if glog.V(3) {
								glog.Infof("[NewDevSessChannel] received:[%v]", string(m))
							}
							p := strings.Split(string(m), ",")
							//p[0]:sentineladr
							//p[1]:mac
							if p[0] != *SentinelAdr {
								if sess, ok := DevSessions.Macs[p[1]]; ok {
									sess.beKilled = true
									if SrvType == CometUdp {
										if sess.offlineEvent != nil {
											sess.offlineEvent.Reset(0)
										}
									}
									if SrvType == CometDw {
										sess.client.Close()
									}
									if glog.V(3) {
										glog.Infof("[NewDevSessChannel] %s %v's offline event done", p[0], p[1])
									}
								} else {
									if glog.V(3) {
										glog.Infof("[NewDevSessChannel] %s %v isn't on the UDPCOMET", p[0], p[1])
									}
								}
							}
						}(n.Data)
					case ZiDevOnlineChannel:
						go func(m []byte) {

							if glog.V(5) {
								glog.Infof("[ZiDevOnline] devid:[%v]", string(m))
							}

							segs := strings.Split(string(m), ",")
							devId, _ := strconv.ParseInt(segs[0], 10, 64)
							srv := segs[1]

							if srv == MEKEY {
								return
							}
							if sess, ok := DevSessions.Sesses[devId]; ok {
								sess.stateLock.Lock()
								defer sess.stateLock.Unlock()
								if subMac, ok := sess.devs[devId]; ok {
									LogWriter.Log(devId, subMac, GDV.getDv(devId), LOGTYPE_LOGOUT)
									go Logout(devId, *SentinelAdr)
									delete(sess.macs, subMac)
									delete(sess.devs, devId)
									//									DelDevAdr([]int64{devId})
									//									PushSubDevOnOfflineMsgToUsers(sess, devId, 1)
									if glog.V(5) {
										glog.Infof("[ZiDevOnline] subdev logout:%s,%v,%s,%v Macs:%v subdev:%d,%s", sess.mac, sess.devId, sess.sid, sess.udpAdr.String(), sess.macs, devId, subMac)
									}
								} else {
									if glog.V(5) {
										glog.Infof("[ZiDevOnline] sub %v no found", devId)
									}
								}
							} else {
								if glog.V(5) {
									glog.Infof("[ZiDevOnline] dev %v no found", devId)
								}
							}
						}(n.Data)
					}
				case redis.Subscription:
					if n.Count == 0 {
						glog.Errorf("Subscription: %s %s %d, %v\n", n.Kind, n.Channel, n.Count, n)
						break a
					}
				case error:
					glog.Errorf("[bind|redis] sub of error: %v\n", data)
					break a
				}
			}
			psc.Close()
			time.Sleep(3 * time.Second)
		}
	}()
}
func handleOnOff(bytes []byte) {
	user_ori := string(bytes)
	users_def := strings.Split(user_ori, "|")
	uid, _ := strconv.ParseInt(users_def[0], 10, 64)
	host := users_def[1]
	isOnline := users_def[2]
	GSentinelMgr.mu.Lock()
	defer GSentinelMgr.mu.Unlock()
	if isOnline == "1" {
		GSentinelMgr.usr2sentinel[uid] = host
		if glog.V(3) {
			glog.Infof("[bus:online] user %d on comet %s", uid, host)
		}
	} else {
		delete(GSentinelMgr.usr2sentinel, uid)
		if glog.V(3) {
			glog.Infof("[bus:offline] user %d on comet %s", uid, host)
		}
	}
}

/**
*	执行lua，更新MSGBUS的用户与WSCOMET映射表，uid可以是硬件或手机用户
 */
func Login(uid int64, host string, mobile string) (bool, error) {
	r := Redix.Get()
	defer r.Close()
	b, err := redis.Bool(ScriptOnline.Do(r,
		fmt.Sprintf(HostPtn, host),
		uid,
		host,
		mobile,
		"",
		"",
		"",
		"",
	))
	if err != nil {
		glog.Errorf("LUA login %d %s %s %v", uid, host, mobile, err)
	} else {
		if glog.V(5) {
			glog.Infof("LUA login %d %s %s", uid, host, mobile)
		}
	}
	return b, err
}

/**
*	执行lua，更新MSGBUS的用户与WSCOMET映射表，uid可以是硬件或手机用户
 */
func Logout(uid int64, host string) error {

	r := Redix.Get()
	defer r.Close()
	_, err := redis.Bool(ScriptOffline.Do(r,
		fmt.Sprintf(HostPtn, host),
		uid,
		host,
		"",
		"",
		"",
	))
	if err != nil {
		glog.Errorf("LUA logout %d %s %v", uid, host, err)
	} else {
		if glog.V(5) {
			glog.Infof("LUA logout %d %s", uid, host)
		}
	}
	return err
}

func ForceUserOffline(uid int64, calm string) {
	r := Redix.Get()
	defer r.Close()

	mobile, _ := redis.String(r.Do("hget", "m:id", uid))
	if mobile != "" {
		if calm == mobile {
			if st, ok := UsrSessions.Sesses[uid]; ok {
				if glog.V(3) {
					glog.Infof("HandleUSROffline usr[%v] only cut off", st.String())
				}
				st.uid = -1
				st.ws.Close()
				return
			}
			if glog.V(3) {
				glog.Infof("HandleUSROffline usr[%v] only cut off %v", uid, calm)
			}
			GSentinelMgr.Forward([]int64{uid}, []byte{10})
			//r.Do("publish", []byte(UserOffline), fmt.Sprint(uid))
		} else {
			if st, ok := UsrSessions.Sesses[uid]; ok {
				_, e := st.ws.Write(AckForceUserOffline)
				if glog.V(3) {
					glog.Infof("HandleUSROffline usr[%v] not only cut off %v but also push C %v", st.String(), mobile, e)
				}

				st.uid = -1
				st.ws.Close()
				return
			}
			if glog.V(3) {
				glog.Infof("HandleUSROffline usr[%v] not only cut off %v but also push C", uid, mobile)
			}
			GSentinelMgr.Forward([]int64{uid}, []byte{11})
		}
	} else {
		if st, ok := UsrSessions.Sesses[uid]; ok {
			if glog.V(3) {
				glog.Infof("HandleUSROffline usr[%v] [%v] [%v]", uid, mobile, calm)
			}
			st.ws.Close()
			return
		}
	}
}
func GetDevLogCode(mac string) string {
	r := Redix.Get()
	defer r.Close()
	code, _ := redis.String(r.Do("hget", []byte("dev:log:code"), mac))
	return code
}
func UpdateDevAdr(sess *DevSession) {
	r := Redix.Get()
	defer r.Close()
	r.Do("hset", DEVADRKEY, fmt.Sprintf("%d", sess.devId), fmt.Sprintf("%d|%s", SrvType, sess.adr))
}
func SetDevAdr(id int64, adr string) {
	r := Redix.Get()
	defer r.Close()
	r.Do("hset", DEVADRKEY, fmt.Sprintf("%d", id), fmt.Sprintf("%d|%s", SrvType, adr))
}
func DelDevAdr(devs []int64) {
	r := Redix.Get()
	defer r.Close()
	for _, id := range devs {
		r.Do("hdel", DEVADRKEY, fmt.Sprintf("%d", id))
	}
}
func PushDevOnlineMsgToUsers(sess *DevSession) {
	r := Redix.Get()
	defer r.Close()
	if len(sess.users) == 0 {
		if glog.V(3) {
			glog.Infof("[PUBDEVONLINEMSG] %s dev {%v} can't send to usr:{%v} for dev online msg, dest is empty", sess.sid, sess.devId, sess.users)
		}
		return
	}
	var DevOnlineMsg []byte
	//	var strIds []string
	//	for _, v := range sess.users {
	//		strIds = append(strIds, fmt.Sprintf("%d", v))
	//	}
	//	userIds := strings.Join(strIds, ",") + "|"
	//	DevOnlineMsg = append(DevOnlineMsg, []byte(userIds)...)
	DevOnlineMsg = append(DevOnlineMsg, byte(9 /**上线标识*/))
	b_buf := bytes.NewBuffer([]byte{})
	binary.Write(b_buf, binary.LittleEndian, sess.devId)
	DevOnlineMsg = append(DevOnlineMsg, b_buf.Bytes()...)
	DevOnlineMsg = append(DevOnlineMsg, byte(1 /**内容长度*/))
	DevOnlineMsg = append(DevOnlineMsg, byte(SrvType))
	msg := &msgs.Msg{}
	msg.FHMask = true
	msg.FHTime = uint32(time.Now().Unix())
	msg.Text = DevOnlineMsg
	msg.FHFin = true
	msg.FHReserve = 0
	msg.FHOpcode = 2
	msg.DHMsgId = uint16(0x35)
	for _, v := range sess.users {
		msg.FHDstId = v
		//			msg.DHKeyLevel = 1
		msg.Msg2Binary()
		GSentinelMgr.Forward([]int64{v}, msg.Final)
	}
	//	r.Do("publish", []byte("PubCommonMsg:0x35"), DevOnlineMsg)
	if glog.V(3) {
		glog.Infof("[PUBDEVONLINEMSG] %s dev[%v] send to usr[%v] for dev online msg, DONE", sess.sid, sess.devId, sess.users)
	}
}
func PushDevOfflineMsgToUsers(sess *DevSession) {
//	r := Redix.Get()
//	defer r.Close()
	for id, _ := range sess.devs {
		r.Do("hdel", DEVADRKEY, fmt.Sprintf("%d", id))
	}
	r.Do("hdel", DEVADRKEY, fmt.Sprintf("%d", sess.devId))
	if len(sess.users) == 0 {
		if glog.V(5) {
			glog.Infof("[PUBDEVOFFLINEMSG] %v dev {%v} can't send to usr:{%v} for dev offline msg, dest is empty", sess.sid, sess.devId, sess.users)
		}
		return
	}
	//	var strIds []string
	//	for _, v := range sess.users {
	//		strIds = append(strIds, fmt.Sprintf("%d", v))
	//	}
	//	userIds := strings.Join(strIds, ",") + "|"

	msg := &msgs.Msg{}
	msg.FHMask = true
	msg.FHTime = uint32(time.Now().Unix())
	msg.FHFin = true
	msg.FHReserve = 0
	msg.FHOpcode = 2
	msg.DHMsgId = uint16(0x35)

	sess.stateLock.Lock()
	defer sess.stateLock.Unlock()
	sess.devs[sess.devId] = sess.mac
	for _, v := range sess.users {
		for id, mac := range sess.devs {
			var DevOfflineMsg []byte
			//			DevOfflineMsg = append(DevOfflineMsg, []byte(userIds)...)
			DevOfflineMsg = append(DevOfflineMsg, byte(10 /**下线标识*/))
			b_buf := bytes.NewBuffer([]byte{})
			binary.Write(b_buf, binary.LittleEndian, id)
			DevOfflineMsg = append(DevOfflineMsg, b_buf.Bytes()...)
			DevOfflineMsg = append(DevOfflineMsg, byte(1 /**内容长度*/))
			DevOfflineMsg = append(DevOfflineMsg, byte(SrvType))

			msg.FHDstId = v
			msg.Text = DevOfflineMsg
			msg.Msg2Binary()
			GSentinelMgr.Forward([]int64{v}, msg.Final)
			//			r.Do("publish", []byte("PubCommonMsg:0x35"), DevOfflineMsg)
			if glog.V(3) {
				glog.Infof("[PUBDEVOFFLINEMSG] %v dev[%v][%v][%v] offline,informing usr%v DONE", sess.sid, sess.devId, id, mac, sess.users)
			}
		}
	}
	delete(sess.devs, sess.devId)
}
func PushSubDevOnOfflineMsgToUsers(sess *DevSession, id int64, flag int) {
	r := Redix.Get()
	defer r.Close()
	if len(sess.users) == 0 {
		if glog.V(5) {
			glog.Errorf("[SUBDEVONOFF] dev[%v] flag[%d] usr%v, dest is empty", id, flag, sess.users)
		}
		return
	}
	var DevOfflineMsg []byte
	//	var strIds []string
	//	for _, v := range sess.users {
	//		strIds = append(strIds, fmt.Sprintf("%d", v))
	//	}
	//	userIds := strings.Join(strIds, ",") + "|"
	//	DevOfflineMsg = append(DevOfflineMsg, []byte(userIds)...)

	if flag == 0 {
		DevOfflineMsg = append(DevOfflineMsg, byte(9 /**上线标识*/))
	} else {
		DevOfflineMsg = append(DevOfflineMsg, byte(10 /**下线标识*/))
	}
	b_buf := bytes.NewBuffer([]byte{})
	binary.Write(b_buf, binary.LittleEndian, id)
	DevOfflineMsg = append(DevOfflineMsg, b_buf.Bytes()...)
	DevOfflineMsg = append(DevOfflineMsg, byte(1 /**内容长度*/))
	DevOfflineMsg = append(DevOfflineMsg, byte(SrvType))

	msg := &msgs.Msg{}
	msg.FHMask = true
	msg.FHTime = uint32(time.Now().Unix())
	msg.FHFin = true
	msg.FHReserve = 0
	msg.FHOpcode = 2
	msg.Text = DevOfflineMsg
	msg.DHMsgId = uint16(0x35)
	for _, v := range sess.users {
		msg.FHDstId = v
		msg.Msg2Binary()
		GSentinelMgr.Forward([]int64{v}, msg.Final)
	}
	//	r.Do("publish", []byte("PubCommonMsg:0x35"), DevOfflineMsg)
	if glog.V(3) {
		glog.Infof("[SUBDEVONOFF] dev[%v] flag[%d]<0:online;1:offline> usr%v DONE", id, flag, sess.users)
	}
}

/**
*返回参数1，与硬件相关的用户列表
*返回参数2，硬件的拥有者
 */
func GetUsersByDev(DevId int64) ([]int64, int64, error) {
	r := Redix.Get()
	defer r.Close()
	user, err := redis.String(r.Do("hget", RedisDeviceUsers, DevId))
	if err != nil {
		return nil, 0xFF, err
	}
	u_id, _ := strconv.ParseInt(user, 10, 64)
	host, err2 := redis.String(r.Do("hget", "user:family", user))
	//如果找不到host，说明此用户是孤儿，那么只返回此设备的直接关联用户
	//如果找到host,就返回此设备直接关联用户所属家庭所有成员
	if host == "" || err2 != nil {
		bindedIds := make([]int64, 0, 1)
		bindedIds = append(bindedIds, int64(u_id))
		return bindedIds, u_id, nil
	} else {
		mems, _ := redis.Strings(r.Do("smembers", fmt.Sprintf("family:%v", host)))
		bindedIds := make([]int64, 0, len(mems))
		for _, m := range mems {
			u_id, err := strconv.ParseInt(m, 10, 64)
			if err == nil {
				bindedIds = append(bindedIds, int64(u_id))
			}
		}
		return bindedIds, int64(u_id), nil
	}
}
func GetDevByUsr(userId int64) ([]int64, error) {
	r := Redix.Get()
	defer r.Close()
	var idStrs []string //字串类型的设备数组
	hostId, err := redis.String(r.Do("hget", "user:family", userId))
	//hostId为空说明此用户为孤儿
	if err != nil || hostId == "" {
		idStrs, _ = redis.Strings(r.Do("smembers", fmt.Sprintf(RedisUserDevices, userId)))
	} else {
		mems, _ := redis.Strings(r.Do("smembers", fmt.Sprintf("family:%v", hostId)))
		for _, m := range mems {
			devs, err := redis.Strings(r.Do("smembers", fmt.Sprintf(RedisUserDevices, m)))
			if err == nil && len(devs) > 0 {
				idStrs = append(idStrs, devs...)
			}
		}
	}
	bindedIds := make([]int64, 0, len(idStrs))
	for _, v := range idStrs {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		bindedIds = append(bindedIds, id)
	}
	return bindedIds, nil
}

func HandleDeviceUsers(buf []byte) {
	if buf == nil {
		return
	}
	if glog.V(3) {
		glog.Infof("[UPDATING BINDING LIST] received [%v]", string(buf))
	}
	strs := strings.SplitN(string(buf), "|", 3)
	if len(strs) != 3 || len(strs) == 0 {
		glog.Errorf("[UPDATING BINDING LIST] invalid pub-sub msg format: %s", string(buf))
		return
	}
	DevId, err := strconv.ParseInt(strs[0], 10, 64)
	if err != nil {
		glog.Errorf("[UPDATING BINDING LIST] invalid DevId %s, error: %v", strs[0], err)
		return
	}
	userId, err := strconv.ParseInt(strs[1], 10, 64)
	if err != nil {
		glog.Errorf("[UPDATING BINDING LIST] invalid userId %s, error: %v", strs[1], err)
		return
	}
	msgType, err := strconv.ParseInt(strs[2], 10, 32)
	if err != nil {
		glog.Errorf("[UPDATING BINDING LIST] invalid bind type %s, error: %v", strs[2], err)
		return
	}
	// new code for udp
	if SrvType == CometUdp {
		go DevSessions.UpdateIds(DevId, userId, msgType != 0)
	}
	if SrvType == CometDw {
		go DevSessions.UpdateIds(DevId, userId, msgType != 0)
	}
	if SrvType == CometWs {
		go UsrSessions.UpdateIds(DevId, userId, msgType != 0)
	}
}

func HandleModifiedPasswd(buf []byte) {
	if buf == nil {
		return
	}
	userId, err := strconv.ParseInt(string(buf), 10, 64)
	if glog.V(3) {
		glog.Infof("[HANDLING MODIFY PWD EVENT] received [%v]", userId)
	}
	if err != nil {
		glog.Errorf("[HANDLING MODIFY PWD EVENT] invalid userId %s, error: %v", string(buf), err)
		return
	}
	go UsrSessions.OffliningUsr(userId)
}
func PubZiDevOnlineChannel(devid string) {
	r := Redix.Get()
	defer r.Close()
	r.Do("publish", []byte(ZiDevOnlineChannel), devid)
}
func PubNewUdpSessChannel(mac, sentineladr string) {
	r := Redix.Get()
	defer r.Close()
	r.Do("publish", NewDevSessChannel, fmt.Sprintf("%s,%s", sentineladr, mac))
	glog.Infof("suicide %s,%s",mac,sentineladr)
}
func GotDevAddr(devId int64) string {
	r := Redix.Get()
	defer r.Close()
	v, _ := redis.String(r.Do("hget", DEVADRKEY, fmt.Sprint(devId)))
	return v
}

/**
* 发送消息到目标，目标可能是手机或硬件
 */
func Send2Dest(msgId uint16, dstIds []int64, msgBody []byte) {
	for _, id := range dstIds {
		if SrvType == CometWs {
			UsrSessions.PushCommonMsg(msgId, id, msgBody)
		} else if SrvType == CometUdp {
			DevSessions.PushCommonMsg(msgId, id, msgBody)
		}
	}
}

func SaveDvName(dvId int64, name string) {
	r := Redix.Get()
	defer r.Close()
	r.Do("hset", []byte("dv:name"), fmt.Sprintf("%v", dvId), name)
}
func GetDvName(dvId int64) string {
	r := Redix.Get()
	defer r.Close()
	DeviceName, _ := redis.String(r.Do("hget", []byte("dv:name"), fmt.Sprintf("%v", dvId)))
	return DeviceName
}
func popOfflineMsg(id int64) (msg []byte) {
	r := Redix.Get()
	defer r.Close()
	msg, _ = redis.Bytes(r.Do("lpop", "msg:"+strconv.Itoa(int(id))))
	return
}
func pushOfflineMsg(id int64, msg []byte) {
	r := Redix.Get()
	defer r.Close()
	r.Do("rpush", "msg:"+strconv.Itoa(int(id)), msg)
}
func setMZ(mac string) {
	r := Redix.Get()
	defer r.Close()
	strMz, err := redis.String(r.Do("get", fmt.Sprintf("MZ:%s", mac)))
	if err != nil {
		if glog.V(5) {
			glog.Infof("mzval fail-------------------%s", err)
		}
	}
	GMZLOCK.Lock()
	defer GMZLOCK.Unlock()
	mz, _ := hex.DecodeString(strMz)
	GMZ[mac] = mz
	if glog.V(5) {
		glog.Infof("mzval--------------%s----------%x", mac, mz)
	}
}

/**
*取设备注册私钥
 */
func sq(id int64) ([]byte, error) {
	var err error
	r := Redix.Get()
	defer r.Close()
	if glog.V(5) {
		glog.Infof("ctlpwd sq:id=%d", id)
	}
	sq, err := redis.Bytes(r.Do("hget", "sq", fmt.Sprintf("%d", id)))
	if err != nil {
		return nil, err
	}
	if glog.V(5) {
		glog.Infof("ctlpwd %d=%x", id, sq)
	}
	return sq, nil
}

/**
*取设备密证
 */
func mz(id int64) ([]byte, error) {
	r := Redix.Get()
	defer r.Close()
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(id))
	if glog.V(5) {
		glog.Infof("mzkey------------------------%x", buf)
	}
	mz, err := redis.String(r.Do("get", "MZ:"+fmt.Sprintf("%x", buf)))
	if err != nil {
		return nil, err
	}
	if glog.V(5) {
		glog.Infof("mzval------------------------%s", mz)
	}
	return hex.DecodeString(mz)
}
func hostName(hostName string) string {
	n := strings.Index(hostName, HostPrefix)
	if n != 0 {
		return hostName
	}
	return hostName[len(HostPrefix):]
}
func ld2cfg(key string) ([]byte, error) {
	r := Redix.Get()
	defer r.Close()
	cfg, err := redis.Bytes(r.Do("hget", "ld2cfg", key))
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
func ld2cfgList() ([]byte, error) {
	var output []byte
	r := Redix.Get()
	defer r.Close()

	mp, er := redis.StringMap(r.Do("hgetall", "ld2cfg"))
	output = append(output, byte(0))
	if er != nil {
		return nil, er
	}
	for _, v := range mp {
		vv := []byte(v)
		o := new(msgs.CFGLD2)
		o.Final = vv
		o.B2O()
		if o.ActionLength > 0 {
			continue
		}
		output[0]++
		output = append(output, o.DevType...)
		output = append(output, o.Version, o.Checksum)
	}

	return output, nil
}

func ld2ds(dmac string) ([]byte, error) {
	devs := make([]byte, 4)
	r := Redix.Get()
	defer r.Close()
	macs, e := redis.Strings(r.Do("smembers", fmt.Sprintf("ld2:ds:%s", dmac)))
	switch {
	case e == nil && len(macs) > 0:
		devs[2] = 0x80 | byte(len(macs)) /**0x80是或*/
		for _, mac := range macs {
			macbyte, _ := hex.DecodeString(mac)
			devs = append(devs, macbyte[4:8]...)
		}
		devs[0] = common.Crc(devs[1:], len(devs[1:]))
		return devs, nil
	case e != nil:
		return nil, e
	case len(macs) == 0:
		return nil, fmt.Errorf("no chain", nil)
	}
	return nil, e
}

func InitUsr2Sentinel() {
	r := Redix.Get()
	defer r.Close()
	hosts, err := redis.Strings(r.Do("keys", "Host:*"))
	if err != nil {
		glog.Errorf("Redis Error, Initing Usr2Sentinel Failed[%v]", err)
		return
	}
	GSentinelMgr.mu.Lock()
	defer GSentinelMgr.mu.Unlock()
	for _, host := range hosts {
		users, err := redis.Strings(r.Do("hkeys", host))
		if err != nil {
			glog.Errorf("Redis Error, Initing Usr2Sentinel Failed[%v]", err)
			continue
		}
		if glog.V(5) {
			glog.Infof("%v users on [%s]", len(users), host)
		}
		for _, user := range users {
			id, err := strconv.ParseInt(user, 10, 60)
			if err != nil {
				if glog.V(5) {
					glog.Errorf("Invaild Id:%v", user)
				}
			} else {
				host = hostName(host)
				GSentinelMgr.usr2sentinel[id] = host
			}
		}
	}
}
func dealDUM(key []byte) {
	r := Redix.Get()
	defer r.Close()
	k := fmt.Sprintf("3507:%x", key)
	r.Do("set", k, k)
}
func getSrvById(id int64) string {
	r := Redix.Get()
	defer r.Close()
	if id < 0 {
		srv, err := redis.String(r.Do("hget", "d:srv", id))
		if err == nil {
			return srv
		} else {
			return ""
		}
	} else {
		srv, err := redis.String(r.Do("hget", "m:srv", id))
		if err == nil {
			return srv
		} else {
			return ""
		}
	}
}
