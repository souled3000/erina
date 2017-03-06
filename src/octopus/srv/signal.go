package srv

import (
	"github.com/golang/glog"
	"os"
	"os/signal"
	"syscall"
)

type closeFunc func()

func HandleSignal(closeF closeFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	//SIGINT    = Signal(0x2)
	//SIGTERM   = Signal(0xf)
	for sig := range c {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			glog.Infof("!!! recv system signal :%v", sig)

			if SrvType == CometDw {
				for _, v := range DevSessions.Macs {
					_ = v.ws.Close()
				}
			}
			if SrvType == CometWs {
				for _, v := range UsrSessions.Sesses {
					_ = v.ws.Close()
				}
			}
			for _, v := range GSentinelMgr.sentinels {
				_ = v.Con.Close()
			}
			closeF()
			return
		}
	}
}
