package srv

import (
	"cloud-base/atomic"
	stat "cloud-base/goprocinfo/linux"
	"cloud-base/procinfo"
	"cloud-base/zk"
	//	"encoding/binary"
	"fmt"
	"github.com/golang/glog"
	zookeeper "github.com/samuel/go-zookeeper/zk"
	//	"io"
	//	"io/ioutil"
	//	"net"
	"strings"
	"time"
)

var (
	zkConn   *zookeeper.Conn
	zkConnOk atomic.AtomicBoolean
	//zkReportCh	chan cometStat
	zkReportCh  chan zookeeper.Event
	zkCometRoot string
)

type cometStat struct {
	Ok   bool
	Path string
	Url  string
}

func init() {
	zkReportCh = make(chan zookeeper.Event, 1)
	zkConnOk.Set(false)
}

func onConnStatus(event zookeeper.Event) {
	switch event.Type {
	case zookeeper.EventSession:
		switch event.State {
		case zookeeper.StateHasSession:
			zkConnOk.Set(true)

		case zookeeper.StateDisconnected:
			fallthrough
		case zookeeper.StateExpired:
			zkConnOk.Set(false)
		}
	}
	zkReportCh <- event
}

func InitZK(zkAddrs []string, udpPath string, wsPath string) {
	if len(udpPath) == 0 {
		glog.Fatalf("[zk] root name for msgbus cannot be empty")
	}
	if len(wsPath) == 0 {
		glog.Fatalf("[zk] root name for comet cannot be empty")
	}
	var (
		//		nodes []string
		err error
		//		conn  *zookeeper.Conn
		//		addr  string
		//		watch <-chan zookeeper.Event
	)
	zkConn, err = zk.Connect(zkAddrs, 60*time.Second, onConnStatus)
	if err != nil {
		glog.Fatal(err)
	}
	if SrvType == CometWs {
		zkCometRoot = "/" + WsPath
	} else {
		zkCometRoot = "/" + UdpPath
	}

	err = zk.Create(zkConn, zkCometRoot)
	if err != nil {
		glog.Infof("[zk] create connection error: %v", err)
	}
	go loop()

}
func updateDev() {
	nodes, _, _, err := zkConn.ChildrenW("/" + UdpPath)
	if err != nil {
		return
	}
	leaves := make(map[string]string, len(nodes))
	for _, node := range nodes {
		addr, err := zk.GetNodeData(zkConn, "/"+UdpPath+"/"+node)
		if err != nil {
			glog.Errorf("[zoo] %s ,%v", addr, err)
			return
		}
		tmp := strings.Split(addr, ",")[5]
		if tmp == *SentinelAdr {
			continue
		}
		leaves[tmp] = ""
		if glog.V(5) {
			glog.Infof("UdpNode:%v Value:%v", node, tmp)
		}
	}
	GSentinelMgr.update(leaves, 1)
}
func updateMob() {
	nodes, _, _, err := zkConn.ChildrenW("/" + WsPath)
	if err != nil {
		return
	}
	leaves := make(map[string]string, len(nodes))
	for _, node := range nodes {
		addr, err := zk.GetNodeData(zkConn, "/"+WsPath+"/"+node)
		if err != nil {
			glog.Errorf("[zoo] %s ,%v", addr, err)
			return
		}
		tmp := strings.Split(addr, ",")[5]
		if tmp == *SentinelAdr {
			continue
		}
		leaves[tmp] = ""
		if glog.V(5) {
			glog.Infof("WsNode:%v Value:%v", node, tmp)
		}
	}
	GSentinelMgr.update(leaves, 0)
}
func CloseZK() {
	if zkConn != nil {
		zkConn.Close()
	}
}

func loop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	cpuChan := procinfo.NewWatcher(time.Minute)
	defer close(cpuChan)

	var cpuUsage float64

	cometPath := make(map[string]cometStat)

	lastTickerTime := time.Now()
	//glog.Infof("[stat|log] loop started %v", lastTickerTime)
	updateMob()
	updateDev()
	for {
		select {
		case event := <-zkReportCh:
			if glog.V(5) {
				glog.Infof("Type:%v,Path:%v,Server:%v,State:%v", event.Type.String(), event.Path, event.Server, event.State.String())
			}
			if event.Path == "/"+WsPath {
				updateMob()
			}
			if event.Path == "/"+UdpPath {
				updateDev()
			}
			if event.Type != zookeeper.EventSession {
				break
			}
			switch event.State {
			case zookeeper.StateHasSession:
				if !zkConnOk.Get() {
					break
				}
				url := ME.String()
				data := onWriteZkData(url, 0.0, 0, 0, 0, SentinelAdr)
				tpath, err := zkConn.Create(zkCometRoot+"/", []byte(data), zookeeper.FlagEphemeral|zookeeper.FlagSequence, zookeeper.WorldACL(zookeeper.PermAll))
				if err != nil {
					glog.Errorf("[zk|comet] create comet node %s with data %s on zk failed: %v", zkCometRoot, data, err)
					break
				}
				if len(tpath) == 0 {
					glog.Errorf("[zk|comet] create empty comet node %s with data %s", zkCometRoot, data)
					break
				}
				cometPath[tpath] = cometStat{Ok: true, Url: ME.String(), Path: tpath}

			case zookeeper.StateDisconnected:
				fallthrough
			case zookeeper.StateExpired:
				cometPath = make(map[string]cometStat)
			}

		//case s := <-zkReportCh:
		//	if len(s.Path) > 0 {
		//		cometPath[s.Path] = s
		//		//glog.Infof("[stat|log] get comet: %v", s)
		//	} else {
		//		cometPath = make(map[string]cometStat)
		//	}

		case cpu, ok := <-cpuChan:
			if !ok {
				cpuUsage = 0.0
				glog.Errorf("[stat|usage] can't get cpu usage from |cpuChan|")
				break
			}
			if cpu.Error != nil {
				cpuUsage = 0.0
				glog.Errorf("[stat|usage] error on get cpu info: %v", cpu.Error)
				break
			}
			cpuUsage = cpu.Usage
			//glog.Infof("[stat|log] get cpu: %f", cpu.Usage)

		case t := <-ticker.C:
			if t.Sub(lastTickerTime) > time.Second*3 {
//				glog.Warningf("[stat|ticker] ticker happened too late, %v after last time", t.Sub(lastTickerTime))
			}
			lastTickerTime = t
			var memTotal uint64
			var memUsage uint64
			meminfo, err := stat.ReadMemInfo("/proc/meminfo")
			if err == nil {
				memTotal = meminfo["MemTotal"]
				memUsage = memTotal - meminfo["MemFree"]
			} else {
				glog.Errorf("[stat|usage] get meminfo from /proc/meminfo failed: %v", err)
			}
			handle := statGetConnOnline()
			glog.Infof("FH:%v", handle)
			for path, s := range cometPath {
				if !s.Ok {
					continue
				}
				if !zkConnOk.Get() {
					glog.Warning("[zk] write zk but conn was broken")
					continue
				}
				data := onWriteZkData(s.Url, cpuUsage, memTotal, memUsage, handle, SentinelAdr)
				_, err := zkConn.Set(path, []byte(data), -1)
				if err != nil {
					glog.Errorf("[zk|comet] set zk node [%s] with comet's status [%s] failed: %v", s.Path, data, err)
				}
				//glog.Infof("[stat|log] write zk path: %s, data: %s", s.Path, data)
			}
		}
	}
}

func onWriteZkData(url string, cpu float64, memTotal uint64, memUsage uint64, handle uint64, sentinel *string) string {
	return fmt.Sprintf("%s,%f,%d,%d,%d,%s,%d", url, cpu, memTotal, memUsage, 1, *sentinel, SrvType)
}
