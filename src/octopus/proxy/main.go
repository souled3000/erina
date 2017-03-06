package main

import (
	"encoding/binary"
	"flag"
	"github.com/garyburd/redigo/redis"
	"github.com/golang/glog"
	"octopus/msgs"
	"octopus/ver"
	"os"
	"runtime"
	//	"strconv"
	"strings"
	"time"
	//	"encoding/hex"
	"fmt"
)

var (
	serverAddr  string
	zkHosts     string
	gProxyRoot  string
	gCometRoot  string
	gRedisAdr   string
	gStatusAddr string
	gCPUUsage   float64
	gMEMFree    float64
	gCount      int64
	printVer    bool
	inverval    time.Duration
	gServer     Server
	gIPS        map[string]string
	gStrIPS     string
	Redix       *redis.Pool
)

func ChooseAUDPServer(mac []byte) (output []byte, addr string) {
	output = make([]byte, 37)
	o := gQueue.Next()

	switch o.(type) {
	case string:
		addr = o.(string)
	default:
		glog.Info("NO UDPCOMET BE AVAILABLE!")
		return nil, ""
	}
	//	glog.Infof("addr:",addr)
	//	glog.Infof("gIPS:",gIPS[addr])
	addr = strings.Replace(addr, addr, gIPS[addr], 1)

	if !strings.HasSuffix(addr, "/ws") {
		addr = "udp://" + addr
	}
	//	adr := strings.Split(addr, ":")
	//	glog.Infof("%v",adr)
	//	if len(adr) != 2 {
	//		glog.Info("WRONG ADR!",adr)
	//		return nil,""
	//	}
	key := RandomByte32()
	copy(output[4:36], key)
	//	r := Redix.Get()
	//	defer r.Close()
	//	r.Do("set","PROXY:"+hex.EncodeToString(mac) ,key)
	//	port, _ := strconv.Atoi(adr[1])
	//	binary.LittleEndian.PutUint16(output[36:38], uint16(port))
	//	output[38] = byte(len(adr[0]))
	//	output = append(output, []byte(adr[0])...)

	output[36] = byte(len(addr))
	output = append(output, []byte(addr)...)

	m := &msgs.Msg{}
	m.Text = output
	m.FHMask = true
	m.FHFin = true
	m.DHAck = true
	m.FHTime = uint32(time.Now().Unix())
	m.FHSrcId = 0x00
	m.FHOpcode = 2
	m.FHDstId = int64(binary.LittleEndian.Uint64(mac))
	m.DHMsgId = 0xc0
	m.DHKeyLevel = 1
	m.DHEncryptType = 2
	m.Msg2Binary()
	output = m.Final
	return
}

func main() {
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	defer glog.Flush()
	glog.CopyStandardLogTo("INFO")

	flag.StringVar(&serverAddr, "serverAddr", "193.168.1.63:7999", "udp proxy ip and port. eg:193.168.1.63:7999")
	flag.StringVar(&gRedisAdr, "redisAdr", "192.168.2.14:6379", "redis address")
	flag.StringVar(&zkHosts, "zks", "193.168.1.221,193.168.1.222,193.168.1.223", "设置ZK的地址,多个地址用逗号分割")
	flag.StringVar(&gCometRoot, "zkrootUdpComet", "CometServersUdp", "zookeeper服务中udp comet所在的根节点名")
	flag.Float64Var(&gCPUUsage, "cpu", 0.9, "CPU最高使用率。如：0.9 表示90%")
	flag.Float64Var(&gMEMFree, "mem", 100, "最小空闲内存大小")
	flag.Int64Var(&gCount, "count", 6000, "最大句柄数")
	//	flag.DurationVar(&inverval, "inv", 10, "轮询UDP服务器状态的时间间隔，s代表秒 ns 代表纳秒 ms 代表毫秒")
	flag.BoolVar(&printVer, "ver", false, "Proxy版本")
	flag.StringVar(&gStrIPS, "ipmap", "193.168.0.60:7999=188.168.0.60:7999|193.168.0.61:7999=188.168.0.60:7999", "代理服务器ip映射表")
	flag.Parse()

	if printVer {
		fmt.Printf("Proxy %s, 插座后台代理服务器.\n", ver.Version)
		return
	}
	glog.Infoln("代理服务器ip映射表", gStrIPS)
	ips := strings.Split(gStrIPS, "|")
	gIPS = make(map[string]string)
	for _, ip := range ips {
		kv := strings.Split(ip, "=")
		gIPS[kv[0]] = kv[1]
	}
	glog.Infoln("代理服务器ip映射表", gIPS)
	initRedix(gRedisAdr)
	go InitZK(strings.Split(zkHosts, ","), gCometRoot)

	gServer = NewServer(serverAddr)
	go gServer.RunLoop()

	handleSignal(func() {
		CloseZK()
		glog.Infof("Closed Server")
	})
}

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
