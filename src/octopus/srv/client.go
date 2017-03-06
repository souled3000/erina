package srv

import (
	"encoding/binary"
	"github.com/golang/glog"
)

func HandleMsg(msg []byte) {
	if len(msg) < 2 {
		glog.Fatalf("Invalidate msg length: [%d bytes] %v", len(msg), msg)
	}
	statIncDownStreamIn()
	size := binary.LittleEndian.Uint16(msg[:2])
	msg = msg[2:]
	data := msg[size*8:]

	for i := uint16(0); i < size; i++ {
		start := i * 8
		id := int64(binary.LittleEndian.Uint64(msg[start : start+8]))
		if SrvType == 0 {
			UsrSessions.PushMsg(id, data)
		} else if SrvType == 1 {
			DevSessions.PushMsg(id, data)
		}
	}
}
