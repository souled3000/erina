package srv

import (
	"fmt"
	"github.com/golang/glog"
	"octopus/msgs"
	"time"
)

func opcode02(t *DevRequest, sess *DevSession) (*DevSession, string, error, string, *msgs.Msg) {
	m := t.Msg
	busi := ""
	ret := &msgs.Msg{}
	statIncUpStreamIn()
	var (
		err error
	)
	if m.DHKeyLevel == 3 {
		m.MZ = GMZ[m.FHSrcMac]
		if m.MZ == nil {
			return nil, m.FHSrcMac, fmt.Errorf("%s has no MZ", m.FHSrcMac), "", nil
		}
		ret.MZ = m.MZ
	}

	err = m.Binary2Msg()
	if err != nil {
		glog.Errorln(err)
		return nil, m.FHSrcMac, err, "", nil
	}
	if glog.V(3) {
		glog.Infoln(m)
	}
	ret.DHMsgId = m.DHMsgId
	ret.FHMask = m.FHMask
	ret.DHAck = true
	ret.DHRead = true
	ret.DHKeyLevel = m.DHKeyLevel
	ret.DHEncryptType = m.DHEncryptType
	ret.FHOpcode = 2
	ret.DHDataSeq = m.DHDataSeq
	if m.DHMsgId != CmdSess && SrvType == CometUdp {
		sess, err = Byte2Sess(m.DHSessionId)
		if err != nil {
			return nil, m.FHSrcMac, err, "", nil
		}
		err = sess.VerifyPack(m.FHSequence)
		if err != nil {
			return nil, m.FHSrcMac, err, "", nil
		}
	}
	switch m.DHMsgId {
	case CmdSess:
		ret.Text, err = onSess(t)
		busi = "CmdSess"
	case CmdRegister:
		ret.Text, err = onRegister(sess, m)
		busi = "CmdRegister"
	case CmdLogin:
		ret.Text, err, _ = onLogin(sess, m)
		busi = "CmdLogin"
	case CmdHeartBeat:
		ret.Text, err = onHearBeat(sess, m)
		busi = "CmdHeartBeat"
	case CmdLogout:
		ret.Text, err = onLogout(sess, m)
		busi = "CmdLogout"
	case CmdLog:
		ret.Text, err = onLog(m)
		busi = "CmdLog"
	case CmdWarn:
		WarnHdl.Deal(m)
		ret.Text = []byte{0x00, 0x00, 0x00, 0x00}
		busi = "CmdWarn"
	case CmdStat:
		StatHdl.Deal(m)
		ret.Text = []byte{0x00, 0x00, 0x00, 0x00}
		busi = "CmdStat"
	case CmdFail:
		FailHdl.Deal(m)
		ret.Text = []byte{0x00, 0x00, 0x00, 0x00}
		busi = "CmdFail"
	case CmdLD2:
		ret.Text, err = onLd2(sess, m)
		busi = "CmdLD2"
	case CmdLD2List:
		ret.Text, err = onLd2List(sess, m)
		busi = "CmdLD2List"
	case CmdLD2Chain:
		ret.Text, err = onLd2Chain(sess, m)
		busi = "CmdLD2Chain"
	case CmdDCM:
		onDcm(sess, m)
		return nil, m.FHSrcMac, fmt.Errorf("CmdDCM don't need to return"), "", nil
	case CmdSL:
		ret.Text, err = onSL(sess, m)
		busi = "CmdSL"
	default:
		return nil, m.FHSrcMac, fmt.Errorf("[02] invalid command type [%v], %v", m.DHMsgId, sess), "", nil
	}
	if err != nil && ret.Text == nil {
		return nil, m.FHSrcMac, fmt.Errorf("[02] CMD:[%x], [%v]", m.DHMsgId, err), "", nil
	}
	if glog.V(5) && err != nil && ret.Text != nil {
		glog.Errorf("[02] CMD:[%x], [%v]", m.DHMsgId, err)
	}

	statIncUpStreamOut()
	if m.DHMsgId != CmdSess {
		ret.DHSessionId = m.DHSessionId
	}
	ret.FHSequence = m.FHSequence
	ret.FHTime = uint32(time.Now().Unix())
	ret.FHSrcId = 0x00
	ret.FHDstId = m.FHSrcId
	ret.Msg2Binary()
	return sess, m.FHSrcMac, nil, busi, ret
}

func opcode03(m *msgs.Msg, sess *DevSession) (*DevSession, string, error) {
	statIncDownStreamIn()
	var (
		err error
	)
	m.MZ, err = mz(m.FHSrcId)
	if err != nil {
		return nil, "", err
	}
	m.Header()
	if m.MZ == nil {
		return nil, m.FHSrcMac, fmt.Errorf("MZ is nil")
	}
	if SrvType == CometUdp {
		if m.FHGuid == nil {
			return nil, m.FHSrcMac, fmt.Errorf("Guid is wrong")
		}
		sess, err = Byte2Sess(m.FHGuid)
		if err != nil {
			return nil, m.FHSrcMac, err
		}
	}
	err = sess.VerifyPack(m.FHSequence)
	if err != nil {
		return nil, m.FHSrcMac, err
	}
	toId := m.FHDstId
	srcId := m.FHSrcId

	msub := &msgs.Msg{}
	msub.Final = m.Final[40:]
	msub.FrameHeader()
	dstId := msub.FHDstId

	if glog.V(5) {
		glog.Infof("%d->%d", sess.devId, dstId)
	}
	destIds := sess.CalcDestIds(dstId)

	if len(destIds) > 0 {
		if glog.V(5) {
			glog.Infof("[03|%d|%s] -> %d by %d,%v;(len:%d)%x", sess.devId, sess.mac, dstId, toId, destIds, len(m.Final), m.Final)
		}
		statIncDownStreamOut()
		GSentinelMgr.Forward(destIds, m.Final)
	} else {
		if glog.V(5) {
			glog.Errorf("[03|%d|%s] from [%x] to [%v] | dst is empty", sess.devId, sess.mac, uint64(srcId), toId)
		}
	}
	return sess, m.FHSrcMac, nil
}

func opcode05(m *msgs.Msg, sess *DevSession) (*DevSession, string, error) {
	statIncDownStreamIn()
	var (
		err error
	)
	m.MZ, err = mz(m.FHSrcId)
	if err != nil {
		return nil, m.FHSrcMac, err
	}
	if m.MZ == nil {
		return nil, m.FHSrcMac, fmt.Errorf("MZ is nil")
	}

	m.Binary2Msg()
	if SrvType == CometUdp {
		sess, err = Byte2Sess(m.DHSessionId)
		if err != nil {
			return nil, m.FHSrcMac, err
		}
		err = sess.VerifyPack(m.FHSequence)
		if err != nil {
			return nil, m.FHSrcMac, err
		}
	}
	toId := m.FHDstId
	srcId := m.FHSrcId
	m.DHKeyLevel = 0
	m.Msg2Binary()

	if glog.V(5) {
		glog.Infof("[05] %d->%d %x", sess.devId, toId, m.Final)
	}
	destIds := sess.CalcDestIds(toId)

	if len(destIds) > 0 {
		if glog.V(5) {
			glog.Infof("[05|%d|%s] -> %d,%v;(len:%d)%x", sess.devId, sess.mac, toId, destIds, len(m.Final), m.Final)
		}
		statIncDownStreamOut()
		GSentinelMgr.Forward(destIds, m.Final)
	} else {
		if glog.V(5) {
			glog.Errorf("[05|%d|%s] from [%x] to [%v] | dst is empty", sess.devId, sess.mac, uint64(srcId), toId)
		}
	}
	return sess, m.FHSrcMac, nil
}
