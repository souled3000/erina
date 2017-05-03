package srv

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"octopus/common/aes"
	"octopus/msgs"
	"strconv"
	"time"

	"github.com/golang/glog"
)

const (
	kHeartBeatSec = 120
	kHeartBeat    = kHeartBeatSec * time.Second
)

func onSess(t *DevRequest) ([]byte, error) {
	m := t.Msg
	body := m.Text
	output := make([]byte, 20)
	if len(body) != 48 {
		return output, fmt.Errorf("[CmdSess err] bad body length %d", len(body))
	}

	mac := make([]byte, 16)
	copy(mac, body[:16])
	// TODO verify mac and sn by real production's information
	key := make([]byte, 32)
	copy(key, body[16:48])

	// if err := VerifyDeviceInfo(mac, sn); err != nil {
	//	return nil, fmt.Errorf("invalid device error: %v", err)
	// }

	//local judgement wether the mac has been existed.

	var s *DevSession

	glog.Infof("CmdSess %s,%x beg SESS", m.FHSrcMac, mac)

	var sid []byte
	if v, ok := DevSessions.Macs[m.FHSrcMac]; ok {
		//		if v.offlineEvent != nil {
		//			v.offlineEvent.Reset(0 * time.Second)
		//		} else {
		//			DevSessions.delSessionByMac(&m.FHSrcMac)
		//		}
		glog.Warningf("CmdSess OLD， %s repeatly invoke OnSess, return current sess :%s", v)
		sid = v.getSid()
	} else {
		//if there isn't mac on local then pushing a msg to cluster.
		PubNewUdpSessChannel(m.FHSrcMac, *SentinelAdr)
		s = newDevSession(t.Peer)
		s.mac = m.FHSrcMac
		sid = s.getSid()
		DevSessions.add(s)
		glog.Infof("CmdSess NEW %s", s)
	}

	binary.LittleEndian.PutUint32(output[:4], 0) //返回结果
	copy(output[4:12], sid)                      //把sid传给板子
	copy(output[12:20], sid)                     //把sid传给板子
	return output, nil
}

func onRegister(sess *DevSession, m *msgs.Msg) ([]byte, error) {
	body := m.Text
	if len(body) < 22 {
		return nil, fmt.Errorf("[REG] %s bad body length %d", sess.sid, len(body))
	}
	url := Urls[m.DHMsgId]
	dv := body[0:4]
	mac := hex.EncodeToString(body[4:12])
	sn := body[12:20]
	doUnbind := int(body[20])
	signature := body[22 : 22+uint32(body[21])]

	input := make(map[string]string)
	input["mac"] = mac
	input["dv"] = fmt.Sprintf("%d", binary.LittleEndian.Uint32(dv))
	input["sn"] = fmt.Sprintf("%x", sn)
	if doUnbind == 1 {
		input["doUnbind"] = "y"
	} else {
		input["doUnbind"] = "n"
	}
	input["sign"] = hex.EncodeToString(signature)

	output := make([]byte, 36)
	httpStatus, rep, err := post(url, input)
	if glog.V(3) {
		glog.Infof("[REG %s %d %s] reg %s, httpstatus:%v,rep:%v,%v", sess.mac, sess.devId, sess.sid, mac, httpStatus, rep, err)
	}
	if err != nil {
		binary.LittleEndian.PutUint32(output[0:4], uint32(httpStatus))
		return output, err
	}

	if s, ok := rep["status"]; ok {
		if status, ok := s.(float64); ok {
			binary.LittleEndian.PutUint32(output[0:4], uint32(int32(status)))
		}
	}
	// device need id in protocol
	if c, ok := rep["cookie"]; ok {
		if cookie, ok := c.(string); ok {
			cki, _ := hex.DecodeString(cookie)
			copy(output[4:36], cki)
		}
	}
	return output, nil
}

func onLogin(sess *DevSession, m *msgs.Msg) ([]byte, error, int32) {
	body := m.Text

	output := make([]byte, 29+4+16+4)
	if len(body) != 42 {
		return output, fmt.Errorf("[Login] %s bad body length %v", sess.sid, len(body)), -1
	}
	url := Urls[m.DHMsgId]
	mac := hex.EncodeToString(body[0:8])

	isFirstLogin := false
	if sess.devId == 0 {
		isFirstLogin = true
	}
	if !isFirstLogin && sess.mac == mac {
		//如果主设备重复登录，直接返回
		return sess.output, nil, -1
	}
	if isFirstLogin && sess.mac != mac {
		return nil, fmt.Errorf("Child %s, your parent %s has not logined.", mac, sess.mac), -1
	}
	dvbigendian := make([]byte, 4)
	//	cookie := string(bytes.TrimRight(body[8:40], "\x00"))
	cookie := hex.EncodeToString(body[8:40])
	input := make(map[string]string)
	input["mac"] = mac
	input["cookie"] = cookie

	httpStatus, rep, err := post(url, input)
	if glog.V(3) {
		glog.Infof("[Login %s %d %s] [%s %s] httpstatus:%v,rep:%v,%v", sess.mac, sess.devId, sess.sid, mac, cookie, httpStatus, rep, err)
	}
	if err != nil {
		binary.LittleEndian.PutUint32(output[0:4], uint32(httpStatus))
		return output, err, -1
	}
	var id int64
	var status int32 = DAckHTTPError
	if s, ok := rep["status"]; ok {
		if n, ok := s.(float64); ok {
			status = int32(n) //status==0 登录成功
			if n == 0 {
				if did, ok := rep["id"]; ok {
					if deid, ok := did.(float64); ok {
						id = int64(deid)

						MAC2ID[mac] = id
						if s, ok := rep["dv"]; ok {
							if sdv, ok := s.(string); ok {
								idv, et := strconv.Atoi(sdv)
								if et == nil {
									dv := uint32(idv)
									binary.LittleEndian.PutUint32(output[33+16:33+16+4], dv)

									binary.BigEndian.PutUint32(dvbigendian, dv)

								}
							}
						}

						if isFirstLogin {
							//it is the first time assigning sess.DevId
							sess.devId = id
							if SrvType == CometDw {
								PubNewUdpSessChannel(sess.mac, *SentinelAdr)
								go func() {
									/**监听设备重复登录**/
									for ke := range sess.killchan {
										if ke == nil {
											return
										}
										switch ke.cmd {
										case 0:
											glog.Infof("%s is killing %s", ke.killer.adr, sess.adr)
											sess.killer = ke.killer
											sess.client.SetLinger(-1)
											sess.client.Close()
										}
									}
								}()
								DevSessions.Devlk.Lock()
								if preSess, ok := DevSessions.Sesses[id]; ok {
									DevSessions.Devlk.Unlock()
									glog.Infof("%s wana kill %s", sess.adr, preSess.adr)
									kickevent := new(KillEvent)
									kickevent.cmd = 0
									kickevent.killer = sess
									preSess.killchan <- kickevent
									<-sess.syncchan
								} else {
									DevSessions.Devlk.Unlock()
								}
								DevSessions.Devlk.Lock()
								sess.sid = fmt.Sprintf("%x", RandomByte8())
								DevSessions.Sids[sess.sid] = sess
								DevSessions.Macs[sess.mac] = sess
								DevSessions.MacSid[sess.mac] = sess.sid
								DevSessions.Devlk.Unlock()

							}

							GDV.writeGDV(id, dvbigendian)
							LogWriter.Log(id, sess.mac, GDV.getDv(id), LOGTYPE_LOGIN)

							DevSessions.Devlk.Lock()
							DevSessions.Sesses[sess.devId] = sess
							Login(sess.devId, *SentinelAdr, "")
							DevSessions.Devlk.Unlock()

							if SrvType == CometUdp {
								assignOffline(sess, err)
							}

							statIncConnTotal()
							statIncConnOnline()

							setMZ(sess.mac)
							sess.mz = GMZ[sess.mac]
							bindedIds, owner, err := GetUsersByDev(sess.devId)
							if err == nil {
								sess.owner = owner
								binary.LittleEndian.PutUint64(output[4:12], uint64(sess.owner))
								if len(bindedIds) > 0 {
									sess.users = bindedIds
								}
							}

							PushDevOnlineMsgToUsers(sess)
							UpdateDevAdr(sess)
						} else if sess.devId == id {
							UpdateDevAdr(sess)
							if glog.V(3) {
								glog.Infof("[%s %v %s] login redundantly.", sess.mac, sess.devId, sess.sid)
							}
							setMZ(sess.mac)
							sess.mz = GMZ[sess.mac]
							//return nil, fmt.Errorf("[%s %v %s] login redundantly.", sess.Mac, sess.DevId, sess.Sid)
						} else if sess.devId != id {
							subMac := hex.EncodeToString(body[0:8])
							if subSess, ok := DevSessions.Sesses[id]; ok {
								if subSess.sid != sess.sid {
									subdevlogout(subSess, id)
								}
							}
							sess.stateLock.Lock()
							if _, ok := sess.devs[id]; !ok {
								GDV.writeGDV(id, dvbigendian)
								PubZiDevOnlineChannel(fmt.Sprintf("%d,%s", id, MEKEY))
								time.Sleep(300 * time.Millisecond)
								sess.devs[id] = subMac
								sess.macs[subMac] = id

								LogWriter.Log(id, subMac, GDV.getDv(id), LOGTYPE_LOGIN)

								DevSessions.Devlk.Lock()
								DevSessions.Sesses[id] = sess
								DevSessions.Devlk.Unlock()

								SetDevAdr(id, sess.adr)
								Login(id, *SentinelAdr, "")
								PushSubDevOnOfflineMsgToUsers(sess, id, 0)
								if glog.V(5) {
									glog.Infof("subdev login: %x,%d  parent %s,%d", body[0:8], id, sess.mac, sess.devId)
								}
							} else {
								if glog.V(5) {
									glog.Infof("subdev login redundantly: %x,%d  parent %s,%d", body[0:8], id, sess.mac, sess.devId)
								}
								//return nil, fmt.Errorf("subdev login redundantly: %x,%d  parent %s,%d", body[0:8], id, sess.Mac, sess.DevId)
							}
							sess.stateLock.Unlock()

						}
					}
				}
				if c, ok := rep["tmp"]; ok {
					if t, ok := c.(string); ok {
						tmp, _ := hex.DecodeString(t)
						copy(output[13:29], tmp)
					}
				}
				if c, ok := rep["ctlpwd"]; ok {
					if t, ok := c.(string); ok {
						glog.Infof("ctlpwd:%s", t)
						tmp, _ := hex.DecodeString(t)
						sq, err := sq(id)
						if err != nil {
							glog.Errorf("ctlpwd sq err:%v", err)
						} else {
							cp, err := aes.EncryptNp(tmp, sq)
							if err != nil {
								glog.Errorf("ctlpwd sq err:%v", err)
							} else {
								glog.Infof("ctlpwd aes:%x", cp)
								copy(output[33:33+16], cp)
							}
						}
					}
				}
			}
		}
	}
	binary.LittleEndian.PutUint32(output[0:4], uint32(status))
	output[12] = byte(UdpTimeout)
	devLogCode := GetDevLogCode(sess.mac)
	if devLogCode != "" {
		dlc, _ := strconv.Atoi(devLogCode)
		binary.LittleEndian.PutUint32(output[29:33], uint32(dlc))
	}
	if isFirstLogin {
		sess.output = output
	}
	if glog.V(5) {
		glog.Infof("[dev:login] [%d %s %s] status:%d,%x", id, mac, sess.sid, status, output)
	}
	return output, nil, status
}
func onSL(sess *DevSession, req *msgs.Msg) ([]byte, error) {
	var (
		id  int64
		ok  bool
		err error
	)
	body := req.Text
	subMac := hex.EncodeToString(body[0:8])
	sess.stateLock.Lock()
	defer sess.stateLock.Unlock()
	if id, ok = sess.macs[subMac]; !ok {
		if id, ok = MAC2ID[subMac]; !ok {
			id, _, err = getDevIdByMac(subMac)
			if err != nil {
				return nil, fmt.Errorf("[err] can't got devid by mac %s,%v", subMac, err)
			}
		}
		PubZiDevOnlineChannel(fmt.Sprintf("%d,%s", id, MEKEY))
		sess.devs[id] = subMac
		sess.macs[subMac] = id

		LogWriter.Log(id, subMac, GDV.getDv(id), LOGTYPE_LOGIN)

		DevSessions.Devlk.Lock()
		DevSessions.Sesses[id] = sess
		DevSessions.Devlk.Unlock()

		SetDevAdr(id, sess.adr)
		Login(id, *SentinelAdr, "")
		PushSubDevOnOfflineMsgToUsers(sess, id, 0)
		if glog.V(5) {
			glog.Infof("subdev login: %x,%d  parent %s,%d", body[0:8], id, sess.mac, sess.devId)
		}
	} else /**如果是子重复登录*/ {
		if glog.V(5) {
			glog.Infof("subdev login redundantly: %x,%d  parent %s,%d", body[0:8], id, sess.mac, sess.devId)
		}
		//return nil, fmt.Errorf("subdev login redundantly: %x,%d  parent %s,%d", body[0:8], id, sess.Mac, sess.DevId)
	}

	out := make([]byte, 12)
	copy(out[4:12], body[0:8])
	return out, nil
}
func onLog(req *msgs.Msg) ([]byte, error) {
	n := int(req.Text[0]) - 1
	output := make([]byte, 8)
	if len(req.Text) != (n+1)*8+1 {
		return nil, fmt.Errorf("req's length = %v, but the number of log is %v", len(req.Text), n)
	}
	binary.LittleEndian.PutUint32(output[0:4], uint32(0))
	copy(output[4:8], req.Text[n*8+1:n*8+4+1])
	LogHdl.Deal(req)
	return output, nil
}
func onHearBeat(sess *DevSession, m *msgs.Msg) ([]byte, error) {
	output := make([]byte, 17)
	binary.LittleEndian.PutUint32(output[:4], 0)
	mac, _ := hex.DecodeString(sess.mac)
	copy(output[4:12], mac)
	binary.LittleEndian.PutUint32(output[12:16], uint32(time.Now().Unix()))
	LogWriter.Log(sess.devId, sess.mac, GDV.getDv(sess.devId), LOGTYPE_HB)
	for k, v := range sess.devs {
		LogWriter.Log(k, v, GDV.getDv(k), LOGTYPE_HB)
	}
	return output, nil
}
func onLd2(sess *DevSession, m *msgs.Msg) ([]byte, error) {
	var output []byte
	body := m.Text
	if len(body) < 6 {
		return nil, fmt.Errorf("0xc8 length error %d,%x", len(body), m.Text)
	}
	//	checksum:=body[4:5]
	version := byte(body[5:6][0])
	key := fmt.Sprintf("%x", binary.LittleEndian.Uint32(body[:4]))
	cfg, err := ld2cfg(key)
	if err != nil {
		if glog.V(5) {
			glog.Infof("[ld2:2007] %x:", key)
		}
		output = make([]byte, 4)
		binary.LittleEndian.PutUint32(output[0:4], 2007)
		return output, err
	}
	if cfg == nil || len(cfg) == 0 {
		if glog.V(5) {
			glog.Infof("[ld2:2008] %x no found:", key)
		}
		output = make([]byte, 4)
		binary.LittleEndian.PutUint32(output[0:4], 2008)
		return output, nil
	}

	o := &msgs.CFGLD2{}
	o.Final = cfg
	o.B2O()
	if version <= o.Version {
		output = make([]byte, len(cfg)+4+2)
		binary.LittleEndian.PutUint32(output[0:4], 0)
		binary.LittleEndian.PutUint16(output[4:6], uint16(len(cfg)))
		copy(output[6:], o.Final)
		return output, nil
	} else {
		output = make([]byte, 4)
		binary.LittleEndian.PutUint32(output[0:4], 2009)
		return output, nil
	}
	return output, nil
}

func onLd2List(sess *DevSession, m *msgs.Msg) ([]byte, error) {
	var output []byte
	cfgList, err := ld2cfgList()
	if err != nil || cfgList[0] == 0 {
		output = []byte{0x01, 0x00, 0x00, 0x00}
		return output, nil
	}
	output = []byte{0x00, 0x00, 0x00, 0x00}
	output = append(output, cfgList...)
	return output, nil
}
func onLd2Chain(sess *DevSession, m *msgs.Msg) ([]byte, error) {
	var output []byte
	body := m.Text
	key := hex.EncodeToString(body[:8])
	cfg, err := ld2ds(key)
	if err != nil {
		if glog.V(5) {
			glog.Infof("[ld2c:2010.0] %x :", key)
		}
		output = make([]byte, 4)
		binary.LittleEndian.PutUint32(output[0:4], 2010)
		return output, err
	}
	if cfg == nil || len(cfg) == 0 {
		if glog.V(5) {
			glog.Infof("[ld2c:2010.1] %x :", key)
		}
		output = make([]byte, 4)
		binary.LittleEndian.PutUint32(output[0:4], 2010)
		return output, nil
	}

	output = make([]byte, len(cfg)+4+2)
	binary.LittleEndian.PutUint32(output[0:4], 0)
	binary.LittleEndian.PutUint16(output[4:6], uint16(len(cfg)))
	copy(output[6:], cfg)
	return output, nil
}

func onLogout(sess *DevSession, m *msgs.Msg) ([]byte, error) {
	body := m.Text
	output := make([]byte, 12)
	copy(output[4:12], body[0:8])
	subMac := fmt.Sprintf("%x", body[0:8])
	sess.stateLock.Lock()
	defer sess.stateLock.Unlock()
	if id, ok := sess.macs[subMac]; !ok {
		binary.LittleEndian.PutUint32(output[:4], 1)
		if glog.V(5) {
			glog.Infof("subdev logout fail:%s,%v,%s,%v Macs:%v Dev:%s", sess.mac, sess.devId, sess.sid, sess.udpAdr.String(), sess.macs, subMac)
		}
		return output, nil
	} else {
		LogWriter.Log(id, subMac, GDV.getDv(id), LOGTYPE_LOGOUT)
		Logout(id, *SentinelAdr)
		delete(sess.macs, fmt.Sprintf("%x", body[0:8]))
		delete(sess.devs, id)
		DelDevAdr([]int64{id})
		PushSubDevOnOfflineMsgToUsers(sess, id, 1)
		if glog.V(5) {
			glog.Infof("subdev logout:%s,%v,%s,%v Macs:%v sub:%d,%s", sess.mac, sess.devId, sess.sid, sess.udpAdr.String(), sess.macs, id, subMac)
		}
	}
	binary.LittleEndian.PutUint32(output[:4], 0)
	return output, nil
}
func onDcm(sess *DevSession, m *msgs.Msg) {
	dealDUM(m.Text)
}

func subdevlogout(sess *DevSession, id int64) {
	sess.stateLock.Lock()
	defer sess.stateLock.Unlock()
	if subMac, ok := sess.devs[id]; ok {
		LogWriter.Log(id, subMac, GDV.getDv(id), LOGTYPE_LOGOUT)
		Logout(id, *SentinelAdr)
		delete(sess.macs, subMac)
		delete(sess.devs, id)
		DelDevAdr([]int64{id})
		PushSubDevOnOfflineMsgToUsers(sess, id, 1)

		DevSessions.Devlk.Lock()
		defer DevSessions.Devlk.Unlock()
		delete(DevSessions.Sesses, id)
		delete(DevSessions.Macs, subMac)

		if glog.V(5) {
			glog.Infof("subdev logout:%s,%v,%s,%v Macs:%v sub:%d,%s", sess.mac, sess.devId, sess.sid, sess.udpAdr.String(), sess.macs, id, subMac)
		}

	} else {
		if glog.V(5) {
			glog.Infof("subdev logout fail:%s,%v,%s,%v Macs:%v Dev:%s", sess.mac, sess.devId, sess.sid, sess.udpAdr.String(), sess.macs, subMac)
		}
	}
}
func assignOffline(s *DevSession, err error) {
	s.offlineEvent = time.AfterFunc(time.Duration(UdpTimeout)*time.Second, func() { devLogout(s, err) })
}

func devLogout(sess *DevSession, err error) {

	if sess.devId == 0 {
		goto final
	}
	PushDevOfflineMsgToUsers(sess)
	DevSessions.Devlk.Lock()
	defer DevSessions.Devlk.Unlock()

	sess.stateLock.Lock()
	for k, v := range sess.devs {
		LogWriter.Log(k, v, GDV.getDv(k), LOGTYPE_LOGOUT)
		Logout(k, *SentinelAdr)
		delete(sess.macs, v)
		delete(sess.devs, k)
		delete(DevSessions.Sesses, k)
		delete(DevSessions.Macs, v)
		if glog.V(5) {
			glog.Infof("subdev logout:%s,%v,%s,%v Macs:%v sub:%d,%s", sess.mac, sess.devId, sess.sid, sess.udpAdr.String(), sess.macs, k, v)
		}
		statDecConnOnline()
	}
	sess.stateLock.Unlock()

	delete(DevSessions.Sesses, sess.devId)
	delete(DevSessions.Sids, sess.sid)
	delete(DevSessions.MacSid, sess.mac)
	delete(DevSessions.Macs, sess.mac)

	if sess.beKilled == false {
		Logout(sess.devId, *SentinelAdr)
	}
	LogWriter.Log(sess.devId, sess.mac, GDV.getDv(sess.devId), LOGTYPE_LOGOUT)
	statDecConnOnline()

final:
	sess.z = time.Now().Unix()
	a := time.Unix(sess.a, 0).Format("2006-01-02 15:04:05")
	b := time.Unix(sess.z, 0).Format("2006-01-02 15:04:05")
	c := sess.lastHeartbeat.Format("2006-01-02 15:04:05")
	if glog.V(3) {
		glog.Infof("[%s %v %s] lasted for [%d]s. from [%s] to [%s], last hearbeat time : %s, %v", sess.mac, sess.devId, sess.sid, sess.z-sess.a, a, b, c, err)
	}
	if sess.killer != nil {
		sess.killer.syncchan <- 0
	}
	if sess.gwclient != nil {
		close(sess.gwclient)
	}
}
