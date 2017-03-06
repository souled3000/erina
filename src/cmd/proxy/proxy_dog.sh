#!/bin/bash

Pwd=$(cd "$(dirname "$0")";pwd)
while :
do
	source $Pwd/proxy.conf
	[[ -d $Pwd/old ]] || mkdir -p $Pwd/old

	#$Pwd/redis-cli -h $onlineRedisIp -p $onlineRedisPort del Host:$cometIp
	#$Pwd/redis-cli -h $onlineRedisIp -p $onlineRedisPort publish $offlineHostKey 0\|$cometIp\|0
	if [ -f $Pwd/${cometName}_update ];then
		mv $Pwd/${cometName} $Pwd/old/${cometName}_`date +%Y%m%d-%H%M%S`
		mv $Pwd/${cometName}_update $Pwd/${cometName}
	fi

echo $serverAddr
echo $inv
echo $cpu
echo $mem
echo $count
echo $zks
echo $zkroot
echo $zkrootUdpComet
echo $ipmap
echo $v

	GOMAXPROCS=1 $Pwd/$cometName -alsologtostderr=true -log_dir="/alidata1/tmp/glog" -serverAddr="$serverAddr"  -redisAdr=$redisAdr -cpu="$cpu" -mem="$mem" -count="$count" -zks="$zks" -zkrootUdpComet="$zkrootUdpComet" -ipmap="$ipmap"  -v=$v
	sleep 3
done

