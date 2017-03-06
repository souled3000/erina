package main

import (
	//"fmt"
	"encoding/binary"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/golang/glog"
	"octopus/msgs"
	"strconv"
	"strings"
	"time"
)

const (
	Hosts                 = "Host:*"
	HostUsers             = "Host:%s"
	OnOff                 = "OnOff"
	SubCommonMsgKeyPrefix = "PubCommonMsg:"
	SubCommonMsgKey       = SubCommonMsgKeyPrefix + "*"
	SubDevCommonMsgKey    = "PubDevCommonMsg:*"
)

var (
	Redix *redis.Pool
)

func initRedix(addr string) {
	Redix = newPool(addr, "")
	r := Redix.Get()
	_, err := r.Do("PING")
	if err != nil {
		glog.Fatalf("Connect Redis [%s] failed [%s]", addr, err.Error())
	}
	r.Close()
	glog.Infof("RedisPool Init OK")
}
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

func InitModel(addr string) {
	initRedix(addr)
	Sub()
}

func Sub() {
	go func() {
		for {
			r := Redix.Get()
			psc := redis.PubSubConn{Conn: r}
			psc.PSubscribe(SubCommonMsgKey, OnOff, SubDevCommonMsgKey)
		a:
			for {
				data := psc.Receive()
				switch n := data.(type) {
				case redis.PMessage:
					switch n.Channel {
					case OnOff:
						go handleOnOff(n.Data)
					case "PubCommonMsg:0x35":
						go handleCommonMsg(n)
					case "PubDevCommonMsg:0x34":
						go handleDevCommonMsg(n)
					}
				case redis.Subscription:
					if n.Count == 0 {
						glog.Errorf("[SUB COMMON MSG EVENT] |%s| |%s| |%d| |%v|", n.Kind, n.Channel, n.Count)
						break a
					}
				case error:
					glog.Errorf("[SUB COMMON MSG EVENT] |%v|", n)
					break a
				}
			}
			psc.Close()
			time.Sleep(3 * time.Second)
		}
	}()
}

func handleCommonMsg(m redis.PMessage) {
	msgid := strings.TrimPrefix(m.Channel, SubCommonMsgKeyPrefix)
	if len(msgid) == 0 {
		return
	}
	mid, err := strconv.ParseUint(msgid, 0, 16)
	if err != nil {
		glog.Errorf("[UCM] Cannot parse wrong MsgId %s, error: %v", msgid, err)
		return
	}

	fields := strings.SplitN(string(m.Data), "|", 2)
	if fields == nil || len(fields) != 2 {
		glog.Errorf("[UCM] invalid pub-sub msg format: %s", string(m.Data))
		return
	}

	msgBody := []byte(fields[1])

	if glog.V(5) {
		glog.Infof("[UCM] REC:%s,%x", fields[0], msgBody)
	}
	
	ids := strings.Split(fields[0], ",")
	dstIds := make([]int64, 0, len(ids))
	for _, i := range ids {
		id, err := strconv.ParseInt(i, 10, 64)
		if err != nil {
			glog.Errorf("[UCM] invalid dest id [%s], %v", i, err)
			return
		}
		if !GSentinelMgr.doesExsist(id) {
			if glog.V(5) {
				glog.Infof("[UCM] %d no found", id)
			}
			return
		}
		dstIds = append(dstIds, id)
		msg := &msgs.Msg{}
		msg.FHMask = true
		msg.FHTime = uint32(time.Now().Unix())
		msg.Text = msgBody
		msg.FHFin = true
		msg.FHReserve = 0
		msg.FHOpcode = 2
		msg.DHMsgId = uint16(mid)
		msg.FHDstId = id
		//			msg.DHKeyLevel = 1
		msg.Msg2Binary()
		GSentinelMgr.Forward([]int64{id}, msg.Final)
	}
	if glog.V(3) {
		glog.Infof("[UCM35-%x] TO %v, CTN: %x", msgBody[0], dstIds, m.Data)
	}
	//		go Send2Dest(uint16(mid), dstIds, msgBody)
}
func handleDevCommonMsg(m redis.PMessage) {
	if glog.V(3) {
		glog.Infof("[DCM] REC:%x", m.Data)
	}
	msgid := strings.TrimPrefix(m.Channel, "PubDevCommonMsg:")
	if len(msgid) == 0 {
		return
	}
	mid, err := strconv.ParseUint(msgid, 0, 16)
	if err != nil {
		glog.Errorf("[DCM] Wrong MsgId %s, error: %v", msgid, err)
		return
	}
	if len(m.Data) < 9 {
		glog.Errorf("[DCM] Wrong Length, len(msg):%v", len(m.Data))
		return
	}
	id := m.Data[:8]
	dst := int64(binary.LittleEndian.Uint64(id))
	if !GSentinelMgr.doesExsist(dst) {
		if glog.V(5) {
			glog.Infof("[DCM] %d no found", dst)
		}
		return
	}
	t := byte(m.Data[8:9][0])
	msg := &msgs.Msg{}
	msg.FHMask = true
	msg.FHTime = uint32(time.Now().Unix())
	msg.Text = m.Data[8:]
	msg.FHOpcode = 2
	msg.DHMsgId = uint16(mid)
	if len(m.Data) >= 17 {
		msg.FHDstId = int64(binary.LittleEndian.Uint64(m.Data[9 : 9+8]))
	} else {
		msg.FHDstId = dst
	}

	msg.DHKeyLevel = 1
	msg.DHEncryptType = 2
	msg.Msg2Binary()
	r := Redix.Get()
	defer r.Close()
	key := fmt.Sprintf("3507:%x", msg.Text)
	glog.Infof("[DCM]key:%s", key)
	n := 0
	for ; n < 5; n++ {
		GSentinelMgr.Forward([]int64{dst}, msg.Final)
		time.Sleep(5 * time.Second)
		v, _ := r.Do("del", key)
		if v == 1 {
			break
		}
	}

	if glog.V(3) {
		glog.Infof("[DCM%d-%d] TO %v CTN:%x Repeat:%d", mid, t, dst, msg.Text, n)
	}
}

func pushOfflineMsg(id int64, msg []byte) {
	r := Redix.Get()
	defer r.Close()
	s := strconv.Itoa(int(id))
	//10 is the length of msgbus frame header
	r.Do("rpush", "msg:"+s, msg[10:])
}
func popOfflineMsg(id int64) (msg []byte) {
	r := Redix.Get()
	defer r.Close()
	s := strconv.Itoa(int(id))
	msg, _ = redis.Bytes(r.Do("lpop", "msg:"+s))
	return
}
func hostName(hostName string) string {
	n := strings.Index(hostName, "Host:")
	if n != 0 {
		return hostName
	}
	return hostName[len("Host:"):]
}
func InitUsr2Sentinel() {
	r := Redix.Get()
	defer r.Close()
	hosts, err := redis.Strings(r.Do("keys", Hosts))
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
			glog.Infof("users%vfrom[%s]", users, host)
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
		if glog.V(5) {
			glog.Infof("[ON] %d %s", uid, host)
		}
	} else {
		delete(GSentinelMgr.usr2sentinel, uid)
		if glog.V(5) {
			glog.Infof("[OF] %d %s", uid, host)
		}
	}
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
