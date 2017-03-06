package srv

import (
//	binary "encoding/binary"
	"encoding/hex"
	"fmt"
	uuid "github.com/nu7hatch/gouuid"
//	"unsafe"
)

var (
//kBalabala = []byte{'h', 'a', 'd', 't', 'o', 'b', 'e', '8'}
)

func NewUuid() *uuid.UUID {
	id, _ := uuid.NewV4()
	return id
}

func Byte2Sess(adr []byte) (*DevSession, error) {
	if adr == nil {
		return nil, fmt.Errorf("SESSION KEY is nil")
	}
	if len(adr) < 8 {
		return nil, fmt.Errorf("SESSION KEY is wrong: %x", adr)
	}
	if sess, ok := DevSessions.Sids[fmt.Sprintf("%x", adr[0:8])]; !ok {
		return nil, fmt.Errorf("No Session[%x]", adr)
		if sess == nil {
			return nil, fmt.Errorf("Corrupt Session[%x]", adr)
		}
	} else {
		return sess, nil
	}
	return nil, nil
}
func Sess2Byte(o *DevSession) []byte {
	//	d := make([]byte, 8)
	//	binary.LittleEndian.PutUint64(d, uint64(uintptr(unsafe.Pointer(o))))
	d, _ := hex.DecodeString(o.sid)
	return d
}

func RandomByte32() []byte {
	var ret []byte
	b1, _ := uuid.NewV4()
	b2, _ := uuid.NewV4()
	ret = append(ret, b1[:]...)
	ret = append(ret, b2[:]...)
	return ret
}
func RandomByte8() []byte {
	ret, _ := uuid.NewV4()
	return ret[0:8]
}
func RandomByte16() []byte {
	ret, _ := uuid.NewV4()
	return ret[:]
}
