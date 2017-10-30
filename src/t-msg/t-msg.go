package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"octopus/common/aes"
	"octopus/msgs"

	"github.com/golang/glog"
)

var ()

func main() {
	flag.Parse()
	glog.CopyStandardLogTo("INFO")
	defer glog.Flush()

	//	fmt.Println(int64(1e6 / 1))
	//	fmt.Println(1e6)
	//	for a := 0; a < 3; a++ {
	//		var b int
	//		fmt.Println(b)
	//		b = b + 1
	//		fmt.Println(b)
	//	}
	//	f()
	binary2msg()
	//	testaes()
}

func begId() {
	mb, _ := hex.DecodeString("fdffffffffffffff")
	a := int64(binary.LittleEndian.Uint64(mb))
	fmt.Println(a)
}

func frameHeader() {
	m := new(msgs.Msg)
	mb, _ := hex.DecodeString("c32e060067e0a25788981f5da8981f5d5167e0a25767e0a2be76c40ab676350ad4795b14eb53f9e3c1ed060067e0a25757675077ca0418a45167e0a25767e0a25162e3a24967ca0df3b696c51f5d4154035e46109873a4c7621e0aac079fa9e5c7ede46af9e44cb5")
	m.Final = mb
	m.FrameHeader()
	fmt.Printf("%s\n", m.String())
}

func f() {
	var e error
	m := new(msgs.Msg)
	mb, _ := hex.DecodeString("8200ea055065046200000000000000000000209148983824020031000a008986ea3b0394bceb43fa6d6407010031013080ea")

	m.Final = mb
	e = m.Binary2Msg()
	fmt.Println(e)
	fmt.Printf("%x\n", m.Final)
	fmt.Printf("%s\n", m.String())

	n := new(msgs.Msg)
	n.FHDstId = 21
	n.FHTime = m.FHTime
	n.FHFin = m.FHFin
	n.FHGuid = m.FHGuid
	n.FHMask = m.FHMask
	n.FHOpcode = m.FHOpcode
	n.FHReserve = m.FHReserve
	n.FHSequence = m.FHSequence
	n.DHSessionId = m.DHSessionId
	n.FHShort = m.FHShort
	n.FHSrcId = m.FHSrcId
	n.DHAck = m.DHAck
	n.DHDataFormat = m.DHDataFormat
	n.DHDataSeq = m.DHDataSeq
	n.DHEncryptType = m.DHEncryptType
	n.DHKeyLevel = m.DHKeyLevel
	n.DHMsgId = m.DHMsgId
	n.DHRead = m.DHRead
	n.DHRead = m.DHRead
	n.Text = m.Text
	n.FHDstId = 0
	n.Msg2Binary()
	fmt.Printf("%x\n", n.Final)
	n.FHDstId = 1
	n.Msg2Binary()
	fmt.Printf("%x\n", n.Final)
	n.FHDstId = 2
	n.Msg2Binary()
	fmt.Printf("%x\n", n.Final)
}
func testaes() {
	key, _ := hex.DecodeString("1bd095db70793c9a6492bc45097d9e4d")
	ori, _ := hex.DecodeString("1bd095db70793c9a6492bc45097d9e4d")
	cipher, e := aes.Encrypt(ori, key)
	fmt.Printf("%x\n", cipher)
	end, e := aes.Decrypt(cipher, key)
	fmt.Printf("%x,%v", end, e)
}
func binary2msg() {
	glog.Infof("%s", "begging")
	m := new(msgs.Msg)
	m.MZ, _ = hex.DecodeString("00000000000000000000000000000000")
	//	m.MZ, _ = hex.DecodeString("e9b2205c80bdfe708e04742a9216e76e")
	//	mb, _ := hex.DecodeString("8500000057cd1a00e9000000000000000000007e56506e2c000030000200c55e00000000000000000001")
	s := "c2000100ffffff01ff000000fe000000ff000000fe0000008ffffeff19ff9f1ea60a17505d25f9219bfecead3a4cff02e96758bad99a5e0dae9a99ed765fcae7"
	mb, _ := hex.DecodeString(s)
	m.Final = mb
	e := m.Binary2Msg()
	fmt.Printf("%v\n", e)
	fmt.Printf("%x\n", m.Final)
	fmt.Printf("%s\n", m)
	m.FHMask = true
	m.Msg2Binary()
	fmt.Println("")
	fmt.Printf("%x\n", m.Final)
	fmt.Printf("%s\n", m)

	resp := new(msgs.Msg)
	resp.Msg2Binary()
}
func cc() {
	glog.Infof("%s", "begging")
	m := new(msgs.Msg)
	mb, _ := hex.DecodeString("c2000400f8130ee1f8130ee1f8130ee1e9131fe1e9131fe17e13cfe1c01352bd129468591cd6d49f019364540a67c1eb80826006cb2a686154d510b03a09ce8509f766843eafe44244a01386098d61184145fc0784f0798a840ffdc2c7c17bcf")
	m.Final = mb
	e := m.Binary2Msg()
	fmt.Printf("%v\n", e)
	fmt.Printf("%s\n", m.String())

	m.FHSrcId = 0x0011001100110011
	m.Msg2Binary()

	fmt.Printf("%s\n", m.String())
}
func Header() {
	m := new(msgs.Msg)
	m.MZ, _ = hex.DecodeString("f867b210aadf1f7393be10f1bbbf5073")
	mb, _ := hex.DecodeString("c3000600bc880100ba880100bc880100bc88b1d521ebf9931f4b283486a8f9e1fbe61935d4c3d6318100050020f1c45706000000000000000000b0d59d63f893ce0072000e00d141bcd1d7a306aa6e99b39d3f346a2bbcf2")
	m.Final = mb
	m.Header()
	fmt.Printf("%s\n", m.String())
	mm := new(msgs.Msg)
	mm.Final = mb[40:]
	mm.Header()
	fmt.Printf("%s\n", mm.String())
}
func Header2() {
	m := new(msgs.Msg)
	m.MZ, _ = hex.DecodeString("e0124f6cf252d26a33957b74ba0d1bf0")
	mb, _ := hex.DecodeString("c3000700568b00001400568b0000568b0000e65e9d63a112e6480c1cb7f4b5aec542cd57eefcd6cec1000400e22cc557d157e22cc557e22cc55752f9583415b58354e12cdb57e03f82d2fe7a718e9d9f80b3bb180ae801e944963b21b401911e9df5ef3c0cd18512")
	m.Final = mb
	m.Header()
	fmt.Printf("%s\n", m.String())
	mm := new(msgs.Msg)
	mm.Final = mb[40:]
	mm.Header()
	fmt.Printf("%s\n", mm.String())
}

func f6() {
	ss, _ := hex.DecodeString("2d35357c303030306230643539643633663661657c31317c307c313438303731383738377c32303136313230333036")
	fmt.Println(string(ss))
	a, _ := hex.DecodeString("c2000f032e6a9c019c012e6a9c012e6a9c019ebf0162d6759201ea6a840151f4699fae56b4b84642f9f89811e84758ae89b11f34c785ced170c525d6c4056d60")
	fmt.Printf("%x\n", a)
	//	mask := []byte{0x11, 0x11, 0x11, 0x11}
	MaskContent(a[8:32], a[4:8])
	fmt.Printf("%x\n", a)
}
func MaskContent(payload []byte, mask []byte) {
	desp := mask[0]
	mask_index := byte(0)
	for iter := uint16(0); iter < uint16(len(payload)); iter++ {
		/* rotate mask and apply it */
		mask_index = byte((iter + uint16(desp)) % 4)
		fmt.Printf("%d:%x\n", mask_index, mask[mask_index])
		payload[iter] ^= mask[mask_index]
	} /* end while */
}
