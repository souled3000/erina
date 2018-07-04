package sentinels

import (
	"errors"
	"time"

	"github.com/fzzy/radix/extra/sentinel"
	"github.com/fzzy/radix/redis"
	"github.com/golang/glog"
)

var (
	ErrNoSentinel = errors.New("all sentinels were not avaliable")
	ErrEmptyName  = errors.New("names of redis cannot be empty")
)

type putChan struct {
	name   string
	client *redis.Client
}

type Sentinels struct {
	addrs     []string
	curIndex  int
	curClient *sentinel.Client
	names     []string
	poolSize  int

	errCh  chan struct{}
	getCh  chan chan *sentinel.Client
	putCh  chan putChan
	downCh chan struct{}
	newCh  chan *sentinel.Client

	quitCh chan struct{}
}

func NewSentinels(addrs []string, poolSize int, names ...string) (*Sentinels, error) {
	if len(addrs) == 0 {
		return nil, ErrEmptyName
	}
	s := &Sentinels{
		addrs:    addrs,
		curIndex: -1,
		names:    names,
		errCh:    make(chan struct{}, 1),
		getCh:    make(chan chan *sentinel.Client, 512),
		putCh:    make(chan putChan, 512),
		downCh:   make(chan struct{}, 1),
		newCh:    make(chan *sentinel.Client),
		quitCh:   make(chan struct{}),
		poolSize: poolSize,
	}
	go s.maker()
	go s.handler()

	s.dial(false)

	client := s.getSentinel(0)
	if client == nil {
		return nil, ErrNoSentinel
	}

	return s, nil
}

func (s *Sentinels) dial(aroundLoop bool) {
	for {
		s.curIndex++
		if s.curIndex >= len(s.addrs) {
			s.curIndex = 0
		}
		client, err := sentinel.NewClient("tcp", s.addrs[s.curIndex], s.poolSize, s.names...)
		if err == nil {
			select {
			case s.newCh <- client:
			default:
				client.Close()
			}
			break
		}
		if (len(s.addrs)-1) == s.curIndex && !aroundLoop {
			break
		}
		time.Sleep(time.Second)
	}
}

func (s *Sentinels) Close() {
	close(s.quitCh)
}

func (s *Sentinels) handler() {
	for {
		select {
		case <-s.quitCh:
			close(s.errCh)
			close(s.getCh)
			close(s.putCh)
			close(s.newCh)
			glog.Errorf("[Sentinels] handler quit")
			return

		case <-s.errCh:
			if s.curClient == nil {
				break
			}

			s.curClient.Close()
			s.curClient = nil

			select {
			case s.downCh <- struct{}{}:
			default:
			}

		case getter := <-s.getCh:
			if s.curClient == nil {
				s.errCh <- struct{}{}
			}
			select {
			case getter <- s.curClient:
			default:
			}

		case putter := <-s.putCh:
			if s.curClient == nil {
				putter.client.Close()
			} else {
				s.curClient.PutMaster(putter.name, putter.client)
			}

		case ns := <-s.newCh:
			s.curClient = ns
		}
	}
}

func (s *Sentinels) maker() {
	for {
		select {
		case <-s.quitCh:
			close(s.downCh)
			glog.Errorf("[Sentinels] maker quit")
			return

		case <-s.downCh:
			s.dial(true)
		}
	}
}

func (s *Sentinels) GetMaster(name string) (*redis.Client, error) {
	return s.GetMasterTimeout(name, 0)
}

func (s *Sentinels) getSentinel(timeout time.Duration) *sentinel.Client {
	ch := make(chan *sentinel.Client, 1)
	var client *sentinel.Client
	if timeout > 0 {
		select {
		case s.getCh <- ch:
		case <-time.After(timeout):
		}
		select {
		case client = <-ch:
		case <-time.After(timeout):
		}
	} else {
		s.getCh <- ch
		client = <-ch
	}
	close(ch)

	return client
}

func (s *Sentinels) GetMasterTimeout(name string, timeout time.Duration) (*redis.Client, error) {
	con, err := s.getMasterTimeout(name, timeout)
	if err != nil {
		if senErr, ok := err.(*sentinel.ClientError); ok && senErr.SentinelErr {
			time.Sleep(time.Second)
			con, err = s.getMasterTimeout(name, timeout)
		}
	}
	return con, err
}

func (s *Sentinels) getMasterTimeout(name string, timeout time.Duration) (*redis.Client, error) {
	client := s.getSentinel(timeout)
	if client == nil {
		return nil, ErrNoSentinel
	}
	con, err := client.GetMaster(name)
	if err != nil {
		if senErr, ok := err.(*sentinel.ClientError); ok && senErr.SentinelErr {
			s.errCh <- struct{}{}
		}
	}
	return con, err
}

func (s *Sentinels) PutMaster(name string, client *redis.Client) {
	s.PutMasterTimeout(name, client, 0)
}

func (s *Sentinels) PutMasterTimeout(name string, client *redis.Client, timeout time.Duration) {
	putter := putChan{name, client}
	if timeout > 0 {
		select {
		case s.putCh <- putter:
		case <-time.After(timeout):
		}
	} else {
		s.putCh <- putter
	}
}
