package main

import (
	"octopus/msgs"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/golang/glog"
)

var (
	Redix *redis.Pool
)

func main() {
	InitRedix("192.168.2.14:6379")
	b, _ := ld2cfgList()
	glog.Infof("%x", b)

	//	mp, er := r.Do("hgetall", "ld2cfg")
	//	if er != nil {
	//		glog.Errorln(er)
	//	}
	//	v := mp.([]interface{})
	//	for _, vv := range v {
	//		glog.Infof("%s", string(vv.([]byte)))
	//		glog.Infof("%x", vv)
	//	}
}
func ld2cfgList() ([]byte, error) {
	var output []byte
	r := Redix.Get()
	defer r.Close()

	mp, er := redis.StringMap(r.Do("hgetall", "ld2cfg"))
	output = append(output, byte(0))
	if er != nil {
		return nil, er
	}
	for _, v := range mp {
		vv := []byte(v)
		o := new(msgs.CFGLD2)
		o.Final = vv
		o.B2O()
		if o.ActionLength == 0 {
			continue
		}
		output[0]++
		output = append(output, o.DevType...)
		output = append(output, o.Version, o.Checksum)
	}

	return output, nil
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

func InitRedix(addr string) {
	Redix = newPool(addr, "")
	r := Redix.Get()
	_, err := r.Do("PING")
	if err != nil {
		glog.Fatalf("Connect Redis [%s] failed [%s]", addr, err.Error())
	}
	defer r.Close()
	glog.Infof("RedisPool Init OK")

}
