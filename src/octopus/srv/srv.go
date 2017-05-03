package srv

import (
	"net"
	"octopus/msgs"
	"sync"
	"time"
)

var (
	MYSQL        string
	UsrSessions  *UsrSessionPool
	ME           *net.TCPAddr
	MEKEY        string
	StatusAddr   string
	UdpPath      string
	WsPath       string
	MbPath       string
	SrvType      CometType
	SentinelAdr  *string
	GMonitor     *Monitor
	UdpTimeout   time.Duration = 125
	RedisAdr     string
	UdpSrv       *UdpServer
	DevSessions  = NewDevSessionPool()
	GSentinelMgr *SentinelMgr
	WarnHdl      Dealer
	LogHdl       Dealer
	StatHdl      Dealer
	FailHdl      Dealer
	LogWriter    Logger
	GMZ          = make(map[string][]byte)
	GDV          = &DVM{lock: &sync.RWMutex{}, m: make(map[int64][]byte)}
	GMZLOCK      = &sync.RWMutex{}
	AUTHURL      string
	Urls         map[uint16]string

	MAC2ID = make(map[string]int64)
)

const (
	LOGTYPE_LOGIN  = 0
	LOGTYPE_LOGOUT = 1
	LOGTYPE_HB     = 2

	CmdSess      = uint16(0xC1)
	CmdRegister  = uint16(0xC2)
	CmdLogin     = uint16(0xC3)
	CmdHeartBeat = uint16(0xC4)
	CmdLogout    = uint16(0xC5)
	CmdWarn      = uint16(0x31)
	CmdStat      = uint16(0x30)
	CmdFail      = uint16(0x32)
	CmdLog       = uint16(0xC6)
	CmdDCM       = uint16(0x34)
	CmdCUM       = uint16(0x35)
	CmdLD2       = uint16(0xC8)
	CmdLD2List   = uint16(0xC9)
	CmdLD2Chain  = uint16(0xCa)
	CmdSL        = uint16(0xCb) //sub dev tell server that he is online
	UrlRegister  = "/api/device/register"
	UrlLogin     = "/api/device/login"

	Code_DEV_ONLINE      = uint8(0x09)
	Code_DEV_OFFLINE     = uint8(0x10)
	Code_DEV_FIRSTONLINE = uint8(0x0d)
)

type CometType uint8

const (
	CometWs CometType = iota
	CometUdp
	CometDw
)

type Dealer interface {
	Deal(*msgs.Msg)
}

type Logger interface {
	Log(id int64, mac string, dv byte, lt int)
}
type DVM struct {
	lock *sync.RWMutex
	m    map[int64][]byte
}

func (this *DVM) writeGDV(id int64, dv []byte) {
	this.lock.Lock()
	defer this.lock.Unlock()
	this.m[id] = dv
}

func (this *DVM) getDv(id int64) byte {
	this.lock.RLock()
	defer this.lock.RUnlock()
	if v, ok := this.m[id]; ok {
		return v[2]
	} else {
		return 0
	}
}
