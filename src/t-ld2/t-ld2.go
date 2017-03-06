package main

import (
	//	"encoding/binary"
	"encoding/hex"
	"fmt"
	"octopus/msgs"
)

func f() {
	o := new(msgs.CFGLD2)
	o.DevType = []byte{0x03, 0x04, 0x05, 0xff}
	o.Version = 0x88
	o.Ldtype = 0x7
	o.AndOr = 0x01
	o.Alarm = make([][]byte, 3)
	o.Alarm[0] = []byte{0x00, 0x00}
	o.Alarm[1] = []byte{0x11, 0x11}
	o.Alarm[2] = []byte{0x21, 0x21}
	o.AlarmLength = byte(len(o.Alarm))
	o.ActionCtn = []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0a}
	o.ActionLength = byte(len(o.ActionCtn) + 1)
	o.ActionPreLocation = 0xcc
	o.O2B()
	fmt.Printf("%v\n", o.ToString())
	fmt.Printf("%x\n", o.Final)
	q := new(msgs.CFGLD2)
	q.Final = o.Final
	q.B2O()
	fmt.Printf("%v\n", q.ToString())
	p := new(msgs.CFGLD2)
	p.ActionCtn = q.ActionCtn
	p.ActionLength = q.ActionLength
	p.ActionPreLocation = q.ActionPreLocation
	p.Alarm = q.Alarm
	p.AlarmLength = q.AlarmLength
	p.AndOr = q.AndOr
	p.DevType = q.DevType
	p.Ldtype = q.Ldtype
	p.Version = q.Version
	p.O2B()
	fmt.Printf("%v\n", p.ToString())
	fmt.Printf("%x\n", p.Final)
}
func main() {
	f2()
}
func f2() {
	a, _ := hex.DecodeString("6b0323006d010402300030013002")
	o := new(msgs.CFGLD2)
	o.Final = a
	fmt.Printf("%x\n", o.Final)
	o.RvsDT()
	fmt.Printf("%x\n", o.Final)
	o.B2O()
	o.O2B()
	fmt.Printf("%x\n", o.Final)

}
