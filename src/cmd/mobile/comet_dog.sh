#!/bin/bash

Pwd=$(cd "$(dirname "$0")";pwd)
redis=/alidata1/services/redis/src
while :
do
	source $Pwd/sys.properties
	[[ -d $Pwd/old ]] || mkdir -p $Pwd/old

    $redis/redis-cli -h localhost -p 6379 del d:srv
    $redis/redis-cli -h localhost -p 6379 del device:adr
    $redis/redis-cli -h localhost -p 6379 del Host:${sentinel}
#	$Pwd/redis-cli -h 192.168.2.14 -p 6379 publish 192.168.2.14-0 0\|192.168.2.14\|0
	if [ -f $Pwd/${cometName}_update ];then
		mv $Pwd/${cometName} $Pwd/old/${cometName}_`date +%Y%m%d-%H%M%S`
		mv $Pwd/${cometName}_update $Pwd/${cometName}
	fi
	GOMAXPROCS=1 $Pwd/$cometName -alsologtostderr=true -log_dir=$glog -v=$glogV
	sleep 3
done

