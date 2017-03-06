#!/bin/bash

Pwd=$(cd "$(dirname "$0")";pwd)
redis=/alidata1/services/redis/src
while :
do
	source $Pwd/sys.properties
	[[ -d $Pwd/old ]] || mkdir -p $Pwd/old

    $redis/redis-cli -h localhost -p 6379 del m:srv
    $redis/redis-cli -h localhost -p 6379 del m:id
    $redis/redis-cli -h localhost -p 6379 del Host:${sentinel}
	if [ -f $Pwd/${cometName}_update ];then
		mv $Pwd/${cometName} $Pwd/old/${cometName}_`date +%Y%m%d-%H%M%S`
		mv $Pwd/${cometName}_update $Pwd/${cometName}
	fi
	echo "-ct=$ct -nettySvr=$nettySvr -me=$me -redis=$redis -zks=$zks -mp=$mp -cp=$cp -v=$glogV -sh=$sh -glogdir=$glog"
	GOMAXPROCS=1 $Pwd/$cometName -alsologtostderr=true -log_dir=$glog -v=$glogV
	sleep 3
done

