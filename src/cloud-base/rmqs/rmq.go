package rmqs

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"cloud-base/hlist"

	"github.com/golang/glog"
	"github.com/streadway/amqp"
)

type Rmqs struct {
	onNew     func(server string)
	onClosed  func(server string)
	onSentMsg func(msg []byte)

	servers *hlist.Hlist
	curr    *hlist.Element
	mu      sync.Mutex
}

func NewRmqs(onNew func(server string), onClosed func(server string), onSentMsg func(msg []byte)) *Rmqs {
	r := &Rmqs{
		onNew:     onNew,
		onClosed:  onClosed,
		onSentMsg: onSentMsg,
		servers:   hlist.New(),
		curr:      nil,
		mu:        sync.Mutex{},
	}
	return r
}

func (this *Rmqs) Add(addr string, exchangeName string) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	for e := this.servers.Front(); e != nil; e = e.Next() {
		s, _ := e.Value.(*rmqServer)
		if s.addr == addr {
			glog.Warningf("[rmq] repeat online server %s", addr)
			return nil
		}
	}

	url := fmt.Sprintf("amqp://%s/", addr)
	conn, err := amqp.Dial(url)
	if err != nil {
		glog.Errorf("[rmq] dial to server %s failed: %v", addr, err)
		return err
	}
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		glog.Errorf("[rmq] get channel from rabbitmq server %s failed: %v", addr, err)
		return err
	}
	closeChan := make(chan *amqp.Error, 1)
	conn.NotifyClose(closeChan)

	err = channel.ExchangeDeclare(
		exchangeName,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		glog.Errorf("[rmq] declare exchange %s failed from rabbitmq server %s failed: %v", exchangeName, addr, err)
		channel.Close()
		conn.Close()
		return err
	}

	s := &rmqServer{
		conn:         conn,
		channel:      channel,
		addr:         addr,
		exchangeName: exchangeName,
	}
	this.servers.PushFront(s)

	if this.curr == nil {
		this.curr = this.servers.Front()
	}

	go this.listenClose(closeChan, s)
	go func() {
		if this.onNew != nil {
			this.onNew(addr)
		}
	}()

	return nil
}

func (this *Rmqs) listenClose(c chan *amqp.Error, s *rmqServer) {
	err := <-c
	glog.Errorf("[rmq|close] close event on rmq server %v, error: %v", s, err)

	addr := s.addr
	exchangeName := s.exchangeName

	this.Remove(s)

	// retry twice connecting to rmq server
	for i := 0; i < 2; i++ {
		time.Sleep(5 * time.Second)
		glog.Infof("[amq|reconnecting] reconnecting %d ...", i+1)
		if nil == this.Add(addr, exchangeName) {
			glog.Infof("[amq|reconnecting] reconnect successful on %d times", i+1)
			return
		}
	}
	glog.Errorf("[amq|reconnecting] reconnect failed")
}

// remove s but not close
func (this *Rmqs) Remove(s *rmqServer) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	for e := this.servers.Front(); e != nil; e = e.Next() {
		if srv, ok := e.Value.(*rmqServer); !ok {
			panic("Invalid type in rmqserver's hlist")
		} else {
			if srv == s {
				glog.Infof("[%s] removed ok", s.addr)
				this.servers.Remove(e)
				go func() {
					if this.onClosed != nil {
						this.onClosed(s.addr)
					}
				}()
				return nil
			}
		}
	}
	return fmt.Errorf("Not exists")
}

func (this *Rmqs) Push(msg []byte, serviceId int) {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.curr != nil {
		c := this.curr.Value.(*rmqServer)
		err := c.channel.Publish(
			c.exchangeName,
			strconv.Itoa(serviceId),
			false,
			false,
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        msg,
			},
		)
		if err == nil {
			if this.onSentMsg != nil {
				this.onSentMsg(msg)
			}
			if glog.V(3) {
				glog.Infof("[rmq|write] write to msgid %d, msg: %s", serviceId, msg)
			} else if glog.V(2) {
				glog.Infof("[rmq|write] write to msgid %d, msg: %s...", serviceId, msg[0:3])
			}

		} else {
			glog.Errorf("[rmq|publish] error on publish msg, error: %v, msg: (%v)", err, msg)
			// 试验性的错误处理，还不确认这个能正确处理服务器关闭的情况
			if err == amqp.ErrClosed {
				go func(server *rmqServer) {
					this.Remove(server)
					server.Close()
					glog.Errorf("[rmq|close] close server %v when pushing msg, error: %v", server, err)
				}(c)
			}
		}
		next := this.curr.Next()
		if next != nil {
			this.curr = next
		} else {
			this.curr = this.servers.Front()
		}
	}
}

type rmqServer struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	addr         string
	exchangeName string
}

func (s *rmqServer) Close() {
	defer s.conn.Close()
	defer s.channel.Close()
	s.addr = ""
}
