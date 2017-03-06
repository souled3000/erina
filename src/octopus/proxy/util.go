package main

import (
	"encoding/binary"
	uuid "github.com/nu7hatch/gouuid"
	"time"
	"octopus/common/aes"
)

var (
//kBalabala = []byte{'h', 'a', 'd', 't', 'o', 'b', 'e', '8'}
)

func NewUuid() *uuid.UUID {
	id, _ := uuid.NewV4()
	return id
}

func RandomByte32() []byte{
	now := uint64(time.Now().Nanosecond())
	btyNow := make([]byte, 8)
	binary.LittleEndian.PutUint64(btyNow, now)
	var ret []byte
	b2, _ := uuid.NewV4()
	r,_:=aes.Encrypt(btyNow,[]byte{0x00,0x01,0x02,0x03,0x04,0x05,0x06,0x07,0x08,0x09,0x0a,0x0b,0x0c,0x0d,0x0e,0x0f})
	ret = append(ret, r[:]...)
	ret = append(ret, b2[:]...)
	return ret
}
