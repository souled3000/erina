package main

import (
	"flag"
	"fmt"
	"net"
	"net/url"
	"octopus/msgs"
	"octopus/srv"
	"octopus/ver"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang/glog"
	p "github.com/magiconair/properties"
)

var (
	kafka    *srv.Server
	logTopic string
)

func main() {
	var e error
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	cfg := p.MustLoadFile("sys.properties", p.UTF8)
	kafAdr := cfg.GetString("kafSvr", "")
	srv.MYSQL = cfg.GetString("mysql", "")
	//ct : comet type
	//	cType := flag.String("ct", "ws", "comet服务类型，可选:1)ws, 2)udp, 3)dw")
	cType := cfg.GetString("ct", "ws")
	//	meadr := flag.String("me", "127.0.0.1:2016", "comet服务器地址")
	meadr := cfg.GetString("me", "127.0.0.1:2015")
	//	srv.SentinelAdr = flag.String("sentinel", "127.0.0.1:2016", "comet服务器转发消息地址")
	guard := cfg.GetString("sentinel", "127.0.0.1:2016")
	srv.SentinelAdr = &guard
	//	nettySvr := flag.String("nettySvr", "http://127.0.0.1:8080", "HTTP服务器根URL(eg: http://127.0.0.1:8080)")
	nettySvr := cfg.GetString("nettySvr", "http://127.0.0.1:8080")
	srv.Urls = make(map[uint16]string)
	srv.Urls[srv.CmdRegister] = nettySvr + srv.UrlRegister
	srv.Urls[srv.CmdLogin] = nettySvr + srv.UrlLogin
	//	flag.StringVar(&srv.RedisAdr, "redis", "193.168.1.224:6379", "Redis服务器地址")
	srv.RedisAdr = cfg.GetString("redis", "127.0.0.1:6379")
	//	zkHosts := flag.String("zks", "193.168.1.221,193.168.1.222,193.168.1.223", "设置ZK的地址,多个地址用逗号分割")
	zkHosts := cfg.GetString("zks", "")
	//	flag.StringVar(&srv.StatusAddr, "sh", ":29999", "程序状态http服务端口")
	srv.StatusAddr = cfg.GetString("sh", ":29999")
	//	flag.StringVar(&srv.UdpPath, "uc", "UdpComet", "zookeeper服务中UDPCOMET所在的根节点名,uc=udp comet")
	srv.UdpPath = cfg.GetString("uc", "UdpComet")
	//	flag.StringVar(&srv.WsPath, "wc", "WebsocketComet", "zookeeper服务中WSCOMET所在的根节点名,wc=websocket comet")
	srv.WsPath = cfg.GetString("wc", "WsComet")
	//	flag.StringVar(&srv.MbPath, "mp", "PushServer", "zookeeper服务中WSCOMET所在的根节点名,mp=msgbus path")
	srv.MbPath = cfg.GetString("mb", "PushServer")
	//	flag.Int64Var(&srv.UdpTimeout, "uto", srv.UdpTimeout, "客户端UDP端口失效时长（秒)")
	srv.UdpTimeout = time.Duration(cfg.GetInt64("uto",int64(srv.UdpTimeout)))
	logTopic = cfg.GetString("logTopic", "his_dev")
	srv.AUTHURL = cfg.GetString("authurl", "")
	printVer := flag.Bool("ver", false, "Comet版本")
	//	serveUdpAddr := flag.String("hhttp", ":8081", "UDP服务器提供HTTP服务的地址")
	flag.Parse()

	srv.ME, e = net.ResolveTCPAddr("tcp", meadr)
	srv.MEKEY = meadr
	if e != nil {
		glog.Fatalln("Svr's adr can't be resolved.")
	}
	glog.Infof("ME %v", srv.ME.String())
	glog.Infof("SentinelAdr %v", *srv.SentinelAdr)
	glog.Infof("kafAdr %v", kafAdr)
	glog.Infof("ct %v", cType)
	glog.Infof("Netty %v", nettySvr)
	glog.Infof("Redis %v", srv.RedisAdr)
	glog.Infof("ZK %v", zkHosts)
	glog.Infof("SH %v", srv.StatusAddr)
	glog.Infof("MP %v", srv.UdpPath)
	if *printVer {
		fmt.Printf("Comet %s, \n", ver.Version)
		return
	}
	switch cType {
	case "ws":
		srv.SrvType = srv.CometWs
	case "udp":
		srv.SrvType = srv.CometUdp
	case "dw":
		srv.SrvType = srv.CometDw
	}
	defer glog.Flush()
	glog.CopyStandardLogTo("INFO")
	srv.GMonitor = &srv.Monitor{Adr: srv.SentinelAdr}
	srv.GMonitor.LaunchMonitor()
	srv.InitStat(srv.StatusAddr)
	srv.InitRedix(srv.RedisAdr)

	if err := srv.ClearRedis(*srv.SentinelAdr); err != nil {
		glog.Fatalf("ClearRedis before starting failed: %v", err)
	}

	go srv.InitZK(strings.Split(zkHosts, ","), srv.UdpPath, srv.WsPath)

	switch srv.SrvType {
	case srv.CometWs:
		kafka = srv.KafkaConsumer(strings.Split(kafAdr, ","), "security_response_topic,security_response_status,security_response_fail,security_hd_log_topic")
		srv.UsrSessions = srv.InitSessionList()
		go srv.LaunchWS()
		go func() {
			for {
				data := <-kafka.ConsumerCh
				m := &msgs.Msg{}
				m.Final = data.Msg
				m.Binary2Msg()
				if glog.V(5){
					glog.Infof("%s %s->%d %x",data.Topic,m.FHSrcMac,m.FHDstId,data.Msg)
				}
				srv.UsrSessions.KafkaPushMsg(m.FHDstId, m.Final)
			}
		}()
	case srv.CometUdp:
		kafka = srv.KafkaProducer(strings.Split(kafAdr, ","))

		if _, e := url.Parse(nettySvr); len(nettySvr) == 0 || e != nil {
			glog.Fatalf("Invalid argument of '-hurl': %s, error: %v", nettySvr, e)
		}
		srv.UdpSrv = &srv.UdpServer{
			Addr: srv.ME.String(),
		}
		srv.WarnHdl = new(WarnHandlerImpl)
		srv.LogHdl = new(LogHandlerImpl)
		srv.StatHdl = new(StatHandlerImpl)
		srv.FailHdl = new(FailHandlerImpl)
		srv.LogWriter = new(LoggerImpl)
		go srv.UdpSrv.RunLoop()
	case srv.CometDw:
		kafka = srv.KafkaProducer(strings.Split(kafAdr, ","))
		srv.WarnHdl = new(WarnHandlerImpl)
		srv.LogHdl = new(LogHandlerImpl)
		srv.StatHdl = new(StatHandlerImpl)
		srv.FailHdl = new(FailHandlerImpl)
		srv.LogWriter = new(LoggerImpl)
		go srv.LaunchDevSrv()
//		go srv.LaunchWS()
//		go srv.StartGW()
	default:
		glog.Fatalf("undifined argument for \"-type\"")
	}

	srv.HandleSignal(func() {
		srv.CloseZK()
		glog.Info("Closed Server")
	})
}

type WarnHandlerImpl struct {
}

func (w *WarnHandlerImpl) Deal(m *msgs.Msg) {
	m.DHKeyLevel = 0
	m.FHMask = false
	m.Msg2Binary()
	m.Binary2Msg()
	km := srv.KafkaMsg{}
	km.Topic = "security_request_topic"
	km.Key = m.FHSrcId
	km.Msg = m.Final

	kafka.ProducerCh <- km
	if glog.V(5) {
		glog.Infof("WARN->KAF %s %x", m.FHSrcMac, m.Final)
	}
}

type LogHandlerImpl struct {
}

func (w *LogHandlerImpl) Deal(m *msgs.Msg) {
	m.DHKeyLevel = 0
	m.FHMask = false
	m.Msg2Binary()
	m.Binary2Msg()
	km := srv.KafkaMsg{}
	km.Topic = "security_hd_log_topic"
	km.Key = m.FHSrcId
	km.Msg = m.Final

	kafka.ProducerCh <- km
	if glog.V(5) {
		glog.Infof("LOG->KAF %s %x", m.FHSrcMac, m.Final)
	}
}

type StatHandlerImpl struct {
}

func (w *StatHandlerImpl) Deal(m *msgs.Msg) {
	m.DHKeyLevel = 0
	m.FHMask = false
	m.Msg2Binary()
	m.Binary2Msg()
	km := srv.KafkaMsg{}
	km.Topic = "security_request_status"
	km.Key = m.FHSrcId
	km.Msg = m.Final

	kafka.ProducerCh <- km
	if glog.V(5) {
		glog.Infof("STAT->KAF %s %x", m.FHSrcMac, m.Final)
	}
}

type FailHandlerImpl struct {
}

func (w *FailHandlerImpl) Deal(m *msgs.Msg) {
	m.DHKeyLevel = 0
	m.FHMask = false
	m.Msg2Binary()
	m.Binary2Msg()
	km := srv.KafkaMsg{}
	km.Topic = "security_request_fail"
	km.Key = m.FHSrcId
	km.Msg = m.Final

	kafka.ProducerCh <- km
	if glog.V(5) {
		glog.Infof("FAIL->KAF %s %x", m.FHSrcMac, m.Final)
	}
}

type LoggerImpl struct {
}

func (w *LoggerImpl) Log(id int64, mac string, dv byte, lt int) {
	now := time.Now()
	ctn := fmt.Sprintf("%d|%s|%d|%d|%d|%s", id, mac, dv, lt, now.Unix(), now.Format("2006010215"))
	if glog.V(5) && lt < 2 {
		glog.Infof("IVAN:%s", ctn)
	}
	km := srv.KafkaMsg{}
	km.Topic = logTopic
	km.Msg = []byte(ctn)
	kafka.ProducerCh <- km
}
