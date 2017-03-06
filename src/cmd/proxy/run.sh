#!/bin/sh

# 直接运行comet的脚本
#nohup ./comet -alsologtostderr=true -log_dir=log -rh=192.168.2.14:6379 -ports=:1234,:1235,:1236,:1237 -lip=192.168.2.14 -zks=192.168.2.221,192.168.2.222,192.168.2.223 -zkroot=MsgBusServers -zkrootc=CometServers -v=3 -sh=":29999" > comet.out 2>&1 &

Pwd=$(cd "$(dirname "$0")";pwd)

source $Pwd/proxy.conf

killall $cometName 
killall proxy_dog.sh

# 运行带清理redis程序的脚本
#nohup ./comet_dog.sh > comet.out 2>&1 &
nohup ./proxy_dog.sh > proxy.out 2>&1 &

