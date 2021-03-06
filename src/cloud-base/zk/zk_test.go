package zk

import (
	"testing"
	"time"
)

func TestZK(t *testing.T) {
	conn, err := Connect([]string{"193.168.1.221:2181"}, time.Second*30)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()
	err = Create(conn, "/test/test")
	if err != nil {
		t.Error(err)
	}
	// registertmp
	err = RegisterTemp(conn, "/test/test", "1")
	if err != nil {
		t.Error(err)
	}
}
