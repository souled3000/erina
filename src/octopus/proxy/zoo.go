package main

import (
	"cloud-base/atomic"
	//	stat "cloud-base/goprocinfo/linux"
	//	"cloud-base/procinfo"
	"cloud-base/zk"
	//	"fmt"
	"github.com/golang/glog"
	zookeeper "github.com/samuel/go-zookeeper/zk"
	"octopus/common"
	"strings"
	"time"
)

var (
	zkConn      *zookeeper.Conn
	zkReportCh  chan zookeeper.Event
	zkConnOk    atomic.AtomicBoolean
	zkCometRoot string
	gQueue      *common.Queue // the data get from zookeeper
)

type (
	proxyStat struct {
		Ok   bool
		Path string
		Url  string
	}
)

func init() {
	gQueue = common.NewQueue()
	zkReportCh = make(chan zookeeper.Event, 1)
	zkConnOk.Set(false)
}

func InitZK(zkAddrs []string, cometName string) {
	if len(cometName) == 0 {
		glog.Fatalf("[zk] root name for comet cannot be empty")
	}
	var (
		err error
	)
	zkConn, err = zk.Connect(zkAddrs, 60*time.Second, onConnStatus)
	if err != nil {
		glog.Fatal(err)
	}

	zkCometRoot = "/" + cometName
	glog.Infof("Connect zk[%v] with udp comet root [%s] OK!", zkAddrs, zkCometRoot)
	UdpCometStat()
}

func UdpCometStat() {
	for {
		zkNodes, _, event, err := zkConn.ChildrenW(zkCometRoot)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		updatingCometQue(zkNodes)
		<-event
		time.Sleep(time.Second)
	}
}

func updatingCometQue(zkNodes []string) {
	var preRetrain []interface{}
	for _, childzode := range zkNodes {
		zdata, err := zk.GetNodeData(zkConn, zkCometRoot+"/"+childzode)
		if err != nil {
			glog.Infof("[%s] cannot get", zdata)
			continue
		}

		items := strings.Split(zdata, ",")
		if len(items) == 7 {
			//			cpuUsage, errT := strconv.ParseFloat(items[1], 64)
			//			if errT != nil {
			//				glog.Infof("get cpu usage err:%v", errT)
			//			}
			//			menTotal, errT := strconv.ParseFloat(items[2], 64)
			//			if errT != nil {
			//				glog.Infof("get total memory err:%v", errT)
			//			}
			//			memUsage, errT := strconv.ParseFloat(items[3], 64)
			//			if errT != nil {
			//				glog.Infof("get  memory usage err:%v", errT)
			//			}
			//			onlineCount, errT := strconv.ParseInt(items[4], 10, 64)
			//			if errT != nil {
			//				glog.Infof("get  online count err:%v", errT)
			//			}
			//			if cpuUsage < gCPUUsage && menTotal-memUsage > gMEMFree && onlineCount < gCount {
			var url string
			switch items[6] {
			case "1":
				url = items[0]
				preRetrain = append(preRetrain, url)
			case "2":
				url = "ws://" + items[0] + "/ws"
				preRetrain = append(preRetrain, url)
			}
			if !gQueue.Contains(url) {
				gQueue.Put(url) //如果满足条件并且队列中没有，则入队
				glog.Infof("add %v", url)
			} else {
				glog.Infof("has been added %v", zdata)
			}
			//			} else {
			//				if gQueue.contains(items[0]) {
			//					gQueue.Remove(items[0]) //如果不满足条件并且队列有，则移除
			//					glog.Infof("del %v", zdata)
			//				} else {
			//					glog.Infof("won't adding %v , because cpu:%v,mem:%v,handler:%v", zdata, cpuUsage, menTotal-memUsage, onlineCount)
			//				}
			//			}
		} else {
			glog.Errorf("err: %s less than 7", zdata)
		}
	}
	//如果znode中没有（说明此udpcomet节点失败），而队列里有则移除
	gQueue.Retrain(preRetrain)
	glog.Infof("remain %v,size:%v", preRetrain, gQueue.Size())
}

func CloseZK() {
	if zkConn != nil {
		zkConn.Close()
	}
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
