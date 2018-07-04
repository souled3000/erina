# Rmqkeeper

RabbitMQ守护程序，通过轮询调用RabbitMQ的心跳测试HTTP接口,检查其是否在线，支持远程检查。

## 安装方法

进入rmqkeeper目录编译:
go build

查看启动参数说明:
./rmqkeeper -help

修改rmqkeeper.conf中的参数，将-h和-url分别设置为RabbitMQ的服务地址和监控地址；设置-zks为Zookeeper集群的IP列表，最后启动程序:
./run.sh
