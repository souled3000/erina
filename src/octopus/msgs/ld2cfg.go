package msgs

import (
	"encoding/binary"
	"fmt"
	"octopus/common"
)

type CFGLD2 struct {
	Checksum          byte
	Version           byte
	Ldtype            byte
	AndOr             byte
	DevType           []byte
	DevTypeRvs        []byte
	AlarmLength       byte
	Alarm             [][]byte
	ActionLength      byte
	ActionPreLocation byte
	ActionCtn         []byte
	Final             []byte
}

func (this *CFGLD2) RvsDT() {
	this.B2O()
	this.DevType = this.Final[4:8]
	tmp := binary.LittleEndian.Uint32(this.DevType)
	binary.BigEndian.PutUint32(this.Final[4:8], tmp)
	for n := byte(0); n < this.AlarmLength; n++ {
		binary.BigEndian.PutUint16(this.Final[8+n*2 : 8+n*2+2],binary.LittleEndian.Uint16(this.Final[8+n*2 : 8+n*2+2]))
	}

}
func (this *CFGLD2) B2O() {
	finalLen := byte(len(this.Final))
	this.Checksum = this.Final[0]
	this.Version = this.Final[1]
	this.Ldtype = (this.Final[2] & 0xe0) >> 5

	this.AndOr = (this.Final[2] & 0x10) >> 4
	this.AlarmLength = this.Final[2] & 0x0f
	this.DevType = this.Final[4:8]

	this.DevTypeRvs = make([]byte, 4)
	tmp := binary.LittleEndian.Uint32(this.DevType)
	binary.BigEndian.PutUint32(this.DevTypeRvs, tmp)

	this.Alarm = make([][]byte, this.AlarmLength)
	for n := byte(0); n < this.AlarmLength; n++ {
		this.Alarm[n] = this.Final[8+n*2 : 8+n*2+2]
	}

	if finalLen > 8+this.AlarmLength*2 {
		this.ActionLength = this.Final[8+this.AlarmLength*2]
		this.ActionPreLocation = this.Final[8+this.AlarmLength*2+1]
		this.ActionCtn = this.Final[8+this.AlarmLength*2+2:]
	}

}
func (this *CFGLD2) O2B() {
	this.Final = make([]byte, 0)
	buf := make([]byte, 0)
	buf = append(buf, this.Version)
	var b3 byte
	b3 |= (this.Ldtype & 0x07) << 5
	b3 |= (this.AndOr & 0x01) << 4
	b3 |= byte(len(this.Alarm))
	buf = append(buf, b3)
	b4 := byte(0)
	buf = append(buf, b4)
	buf = append(buf, this.DevType[0:]...)
	for _, code := range this.Alarm {
		buf = append(buf, code[:]...)
	}
	if this.ActionLength > 0 {
		buf = append(buf, this.ActionLength)
		buf = append(buf, this.ActionPreLocation)
		buf = append(buf, this.ActionCtn...)
	}
	checksum := common.Crc(buf, len(buf))
	this.Final = append(this.Final, checksum)
	this.Final = append(this.Final, buf...)
}
func (this *CFGLD2) ToString() string {
	s := fmt.Sprintf("DevType:%x\n", this.DevType)
	s += fmt.Sprintf("DevTypeRvs:%x\n", this.DevTypeRvs)
	s += fmt.Sprintf("Ldtype:%x\n", this.Ldtype)
	s += fmt.Sprintf("Version:%x\n", this.Version)
	s += fmt.Sprintf("AndOr:%x\n", this.AndOr)
	s += fmt.Sprintf("AlarmLength:%x\n", this.AlarmLength)
	s += fmt.Sprintf("Alarms:%x\n", this.Alarm)
	s += fmt.Sprintf("ActionLength:%x\n", this.ActionLength)
	s += fmt.Sprintf("ActionPreLocation:%x\n", this.ActionPreLocation)
	s += fmt.Sprintf("ActionCtn:%x\n", this.ActionCtn)
	return s
}
