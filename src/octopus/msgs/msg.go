package msgs

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	common "octopus/common"
	aes "octopus/common/aes"
	//	"strconv"
	//	"strings"
	//	"time"
	"time"

	"github.com/golang/glog"
)

var ()

const (
	HEADERCRCIDX   = 30
	BODYCRCIDX     = 31
	FrameHeaderLen = 24
	DataHeaderLen  = 16
	HeaderLen      = FrameHeaderLen + DataHeaderLen
	CIPHERIDX      = FrameHeaderLen + 8
	GUIDLEN        = 16

	// 应用层协议的消息ID
	MIDOnlineOffline = 0x34
	MIDBindUnbind    = 0x35
	MIDStatus        = 0x36
)

type encodec interface {
	Msg2Binary()
	Binary2Msg() error
}

type Msg struct {
	//密证
	MZ []byte
	/** -----------------------FRAME HEADER---------------------------- */

	FHFin    bool
	FHMask   bool
	FHVer    byte
	FHShort  bool
	FHOpcode byte

	FHReserve byte

	FHSequence uint16

	FHTime uint32

	FHDstId  int64
	FHDstMac string

	FHSrcId  int64
	FHSrcMac string
	FHGuid   []byte

	/** ------------------------DATA HEADER--------------------------- */

	DHRead        bool
	DHAck         bool
	DHDataFormat  byte
	DHKeyLevel    byte
	DHEncryptType byte

	DHDataSeq byte

	DHMsgId uint16

	DHLength uint16

	DHHeadCheck byte

	DHCheck byte

	/** -------------------------BODY-----------------------------------*/
	DHSessionId []byte

	/**------------------------------------------------------------*/
	datacrc []byte
	Text    []byte
	Final   []byte
	RK      []byte
	RKXORMZ []byte
	CIPHER  []byte
}

func (m *Msg) String() string {
	var txt string
	txt += fmt.Sprintf("length:%d|", len(m.Final))
	txt += fmt.Sprintf("FHMask:%v|", m.FHMask)
	txt += fmt.Sprintf("FHOpcode:%x|", m.FHOpcode)
	txt += fmt.Sprintf("DHMsgId:%x|", m.DHMsgId)
	txt += fmt.Sprintf("FhSrcMac:%s|", m.FHSrcMac)
	txt += fmt.Sprintf("FhDstMac:%s|", m.FHDstMac)
	txt += fmt.Sprintf("FhSrcId:%x|", uint64(m.FHSrcId))
	txt += fmt.Sprintf("FHDstId:%x|", uint64(m.FHDstId))
	txt += fmt.Sprintf("DHSessionId:%x|", m.DHSessionId)
	txt += fmt.Sprintf("FHGuid:%x|", m.FHGuid)
	txt += fmt.Sprintf("FHSequence:%x|", m.FHSequence)
	txt += fmt.Sprintf("FHTime:%x|", m.FHTime)
	txt += fmt.Sprintf("DHKeyLevel:%x|", m.DHKeyLevel)
	txt += fmt.Sprintf("DHLength:%d|", m.DHLength)
	txt += fmt.Sprintf("DHHeadCheck:%x|", m.DHHeadCheck)
	txt += fmt.Sprintf("DHCheck:%x|", m.DHCheck)
	txt += fmt.Sprintf("DHAck:%v|", m.DHAck)
	txt += fmt.Sprintf("Final:%x|", m.Final)
	if m.MZ != nil {
		txt += fmt.Sprintf("MZ:%x|", m.MZ)
	}
	if m.RK != nil {
		txt += fmt.Sprintf("RK:%x|", m.RK)
	}
	if m.RKXORMZ != nil {
		txt += fmt.Sprintf("RKXORMZ:%x|", m.RKXORMZ)
	}
	if m.CIPHER != nil {
		txt += fmt.Sprintf("CIPHER:%x|", m.CIPHER)
	}
	txt += fmt.Sprintf("Text:%x|", m.Text)
	txt += fmt.Sprintf("datacrc:%x", m.datacrc)
	return txt
}
func (m *Msg) FrameHeader() error {
	mlen := len(m.Final)
	if mlen == 0 {
		return fmt.Errorf("[MSG] The msg is empty.")
	}
	m.FHOpcode = (0x07 & m.Final[0])
	if m.FHOpcode != 2 && m.FHOpcode != 3 && m.FHOpcode != 1 && m.FHOpcode != 5 {
		return fmt.Errorf("[MSG] wrong opcode, op=%v", m.FHOpcode)
	}
	if (m.FHOpcode != 3 && mlen < FrameHeaderLen+DataHeaderLen) || (m.FHOpcode == 3 && mlen < FrameHeaderLen+DataHeaderLen+GUIDLEN) {
		return fmt.Errorf("[MSG] The length of msg is at least  40 when opcode is 2 or 48 when opcode is 3,  mlen=%v ,opcode= %v", mlen, m.FHOpcode)
	}
	if m.Final[0]&0x80 == 0x80 {
		m.FHFin = true
	}
	if m.Final[0]&0x40 == 0x40 {
		m.FHMask = true
	}
	m.FHVer = (m.Final[0] & 0x30) >> 4
	if m.Final[0]&0x08 == 0x08 {
		m.FHShort = true
	}

	m.FHReserve = m.Final[1]

	m.FHSequence = binary.LittleEndian.Uint16(m.Final[2:4])

	m.FHTime = binary.LittleEndian.Uint32(m.Final[4:8])

	if m.FHMask {
		switch m.FHOpcode {
		case 0x03:
			MaskContent(m.Final[8:40], m.Final[4:8])
		default:
			MaskContent(m.Final[8:32], m.Final[4:8])
		}

		defer func() {
			switch m.FHOpcode {
			case 0x03:
				MaskContent(m.Final[8:40], m.Final[4:8])
			default:
				MaskContent(m.Final[8:32], m.Final[4:8])
			}
		}()
	}

	m.FHDstId = int64(binary.LittleEndian.Uint64(m.Final[8:16]))
	m.FHDstMac = hex.EncodeToString(m.Final[8:16])

	m.FHSrcId = int64(binary.LittleEndian.Uint64(m.Final[16:24]))
	m.FHSrcMac = hex.EncodeToString(m.Final[16:24])
	return nil
}
func (m *Msg) Header() error {
	mlen := len(m.Final)
	if mlen == 0 {
		return fmt.Errorf("[MSG] The msg is empty.")
	}
	m.FHOpcode = (0x07 & m.Final[0])
	if m.FHOpcode != 2 && m.FHOpcode != 3 && m.FHOpcode != 1 && m.FHOpcode != 5 {
		return fmt.Errorf("[MSG] wrong opcode, op=%v", m.FHOpcode)
	}
	if (m.FHOpcode != 3 && mlen < FrameHeaderLen+DataHeaderLen) || (m.FHOpcode == 3 && mlen < FrameHeaderLen+DataHeaderLen+GUIDLEN) {
		return fmt.Errorf("[MSG] The length of msg is at least  40 when opcode is 2 or 48 when opcode is 3,  mlen=%v ,opcode= %v", mlen, m.FHOpcode)
	}
	if m.Final[0]&0x80 == 0x80 {
		m.FHFin = true
	}
	if m.Final[0]&0x40 == 0x40 {
		m.FHMask = true
	}
	m.FHVer = (m.Final[0] & 0x30) >> 4
	if m.Final[0]&0x08 == 0x08 {
		m.FHShort = true
	}

	m.FHReserve = m.Final[1]

	m.FHSequence = binary.LittleEndian.Uint16(m.Final[2:4])

	m.FHTime = binary.LittleEndian.Uint32(m.Final[4:8])
	if m.FHMask {
		switch m.FHOpcode {
		case 0x03:
			MaskContent(m.Final[8:40], m.Final[4:8])
		default:
			MaskContent(m.Final[8:32], m.Final[4:8])
		}

		defer func() {
			switch m.FHOpcode {
			case 0x03:
				MaskContent(m.Final[8:40], m.Final[4:8])
			default:
				MaskContent(m.Final[8:32], m.Final[4:8])
			}
		}()
	}

	m.FHDstId = int64(binary.LittleEndian.Uint64(m.Final[8:16]))
	m.FHDstMac = hex.EncodeToString(m.Final[8:16])
	m.FHSrcId = int64(binary.LittleEndian.Uint64(m.Final[16:24]))
	m.FHSrcMac = hex.EncodeToString(m.Final[16:24])
	offset := FrameHeaderLen
	if m.FHOpcode == 3 {
		offset = FrameHeaderLen + GUIDLEN
		m.DHKeyLevel = (m.Final[offset] & 0x0c) >> 2
		//		switch m.DHKeyLevel {
		//		case 0:
		//			m.FHGuid = m.Final[FrameHeaderLen : offset-8]
		//		case 1:
		//		case 2:
		//		case 3:
		//		default:
		if m.MZ != nil {
			a := RandomKeyGet(m.FHSequence, m.FHSrcId, m.FHTime)
			//			a := RandomKeyGet(m.FHSrcId, m.FHTime)
			m.RK = a
			key := make([]byte, 16)
			for n, x := range a {
				key[n] = x ^ m.MZ[n]
			}
			aa, err := aes.Decrypt(m.Final[FrameHeaderLen:offset], key)
			if err != nil {
				if glog.V(6) {
					glog.Infof("%s,%v", err, m)
				}
			}
			m.FHGuid = aa
		} else {
			return fmt.Errorf("MZ is nil but Opcode is 3,%x", m.Final)
		}
		//		}
	}
	m.DHKeyLevel = (m.Final[offset] & 0x0c) >> 2
	m.DHMsgId = binary.LittleEndian.Uint16(m.Final[offset+2 : offset+4])

	return nil
}
func (m *Msg) Binary2Msg() error {
	if m.Final == nil || len(m.Final) == 0 {
		return fmt.Errorf("[MSG] The msg is empty.")
	}

	mlen := len(m.Final)

	m.FHOpcode = (0x07 & m.Final[0])
	if m.FHOpcode != 2 && m.FHOpcode != 3 && m.FHOpcode != 1 && m.FHOpcode != 5 {
		return fmt.Errorf("[MSG] wrong opcode, op=%v", m.FHOpcode)
	}
	if (m.FHOpcode != 3 && mlen < FrameHeaderLen+DataHeaderLen) || (m.FHOpcode == 3 && mlen < FrameHeaderLen+DataHeaderLen+GUIDLEN) {
		return fmt.Errorf("[MSG] The length of msg is at least  40 when opcode is 2 or 48 when opcode is 3,  mlen=%v ,opcode= %v", mlen, m.FHOpcode)
	}
	if m.Final[0]&0x80 == 0x80 {
		m.FHFin = true
	}
	if m.Final[0]&0x40 == 0x40 {
		m.FHMask = true
	}
	m.FHVer = (m.Final[0] & 0x30) >> 4
	if m.Final[0]&0x08 == 0x08 {
		m.FHShort = true
	}

	m.FHReserve = m.Final[1]

	m.FHSequence = binary.LittleEndian.Uint16(m.Final[2:4])

	m.FHTime = binary.LittleEndian.Uint32(m.Final[4:8])

	if m.FHMask {
		switch m.FHOpcode {
		case 0x03:
			MaskContent(m.Final[8:40], m.Final[4:8])
		default:
			MaskContent(m.Final[8:32], m.Final[4:8])
		}
		defer func() {
			switch m.FHOpcode {
			case 0x03:
				MaskContent(m.Final[8:40], m.Final[4:8])
			default:
				MaskContent(m.Final[8:32], m.Final[4:8])
			}
		}()
	}

	m.FHDstId = int64(binary.LittleEndian.Uint64(m.Final[8:16]))
	m.FHSrcId = int64(binary.LittleEndian.Uint64(m.Final[16:24]))
	m.FHDstMac = hex.EncodeToString(m.Final[8:16])
	m.FHSrcMac = hex.EncodeToString(m.Final[16:24])

	offset := FrameHeaderLen
	if m.FHOpcode == 3 {
		offset = FrameHeaderLen + GUIDLEN
		m.FHGuid = m.Final[FrameHeaderLen:offset]
	}

	if m.Final[offset]&0x80 == 0x80 {
		m.DHRead = true
	}
	if m.Final[offset]&0x40 == 0x40 {
		m.DHAck = true
	}
	m.DHDataFormat = (m.Final[offset] & 0x30) >> 4
	m.DHKeyLevel = (m.Final[offset] & 0x0c) >> 2
	m.DHEncryptType = m.Final[offset] & 0x03

	m.DHDataSeq = m.Final[offset+1]
	m.DHMsgId = binary.LittleEndian.Uint16(m.Final[offset+2 : offset+4])
	m.DHLength = binary.LittleEndian.Uint16(m.Final[offset+4 : offset+6])
	if int(m.DHLength) != len(m.Final[offset+16:]) { //报文实际长度与报文体内设置的长度不一致，这个长度等于m.txt加密后的长度+m.DHSessionID的长度也就是8
		return fmt.Errorf("[MSG:02] wrong body length in data header: %d != %d", m.DHLength, len(m.Final[offset+16:]))
	}
	m.DHHeadCheck = m.Final[offset+6]
	m.DHCheck = m.Final[offset+7]

	// discard msg if found checking error
	if m.DHHeadCheck != common.Crc(m.Final, offset+6) {
		return fmt.Errorf("[MSG:02] checksum header error %x!=%x", m.DHHeadCheck, common.Crc(m.Final, offset+6))
	}
	switch m.DHKeyLevel {
	case 0:
		m.DHSessionId = m.Final[offset+8 : offset+16]
		if m.DHCheck != common.Crc(m.Final[offset+8:], len(m.Final[offset+8:])) {
			return fmt.Errorf("body crc error %x!=%x", m.DHCheck, common.Crc(m.Final[offset+8:], len(m.Final[offset+8:])))
		}
		if m.DHLength > 0 {
			m.Text = m.Final[offset+DataHeaderLen:]
		}
	case 1:
		key := RandomKeyGet(m.FHSequence, m.FHSrcId, m.FHTime)
		//		key := RandomKeyGet(m.FHSrcId, m.FHTime)
		m.RK = key
		aa, err := aes.Decrypt(m.Final[offset+8:], key)
		if err != nil {
			return fmt.Errorf("[%v]%v", err, m)
		}
		if len(aa) < 8 {
			m.Text = aa
			return fmt.Errorf("too short,text:%v|%x", len(aa), aa)
		}
		if m.DHCheck != common.Crc(aa, len(aa)) {
			return fmt.Errorf("body crc error %v!=%v|%x|%v", m.DHCheck, common.Crc(aa, len(aa)), aa, len(aa))
		}
		m.DHSessionId = aa[:8]
		m.Text = aa[8:]
	case 2:
	case 3:
		if m.MZ != nil {
			a := RandomKeyGet(m.FHSequence, m.FHSrcId, m.FHTime)
			m.RK = a
			key := make([]byte, 16)
			for n, x := range a {
				key[n] = x ^ m.MZ[n]
			}
			m.RKXORMZ = key
			m.CIPHER = m.Final[offset+8:]
			aa, err := aes.Decrypt(m.Final[offset+8:], key)
			if err != nil {
				return fmt.Errorf("[%s]%v", err, m)
			}
			if len(aa) < 8 {
				m.Text = aa
				return fmt.Errorf("too short,text:%v|%x", len(aa), aa)
			}
			if m.DHCheck != common.Crc(aa, len(aa)) {
				return fmt.Errorf("body crc error %v!=%v|%x|%v", m.DHCheck, common.Crc(aa, len(aa)), aa, len(aa))
			}
			m.DHSessionId = aa[:8]
			if len(aa) > 8 {
				m.Text = aa[8:]
			}
		} else {
			return fmt.Errorf("MZ is nil but Opcode is %d", m.FHOpcode)
		}
	default:
		return fmt.Errorf("[MSG:02] wrong KeyLevel %v", m.DHKeyLevel)
	}
	return nil
}

//发送时用
func (m *Msg) Msg2Binary() error {
	m.Final = make([]byte, 0)
	offset := FrameHeaderLen
	if m.FHOpcode == 3 {
		offset = FrameHeaderLen + GUIDLEN
	}
	buf := make([]byte, offset+DataHeaderLen)

	if m.FHFin {
		buf[0] |= 0x80 // 1<<7
	}
	if m.FHMask {
		buf[0] |= 0x40 // 1<<6
	}
	if m.FHVer != 0 {
		buf[0] |= (m.FHVer & 0x3) << 4
	}
	if m.FHShort {
		buf[0] |= 0x08
	}

	buf[0] |= m.FHOpcode & 0x7

	buf[1] = m.FHReserve

	binary.LittleEndian.PutUint16(buf[2:4], m.FHSequence)
	if m.FHTime == 0 {
		a := time.Now().UnixNano() / 1e6
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(a))
		m.FHTime = binary.BigEndian.Uint32(b[0:4])
	}
	binary.LittleEndian.PutUint32(buf[4:8], m.FHTime)

	binary.LittleEndian.PutUint64(buf[8:16], uint64(m.FHDstId))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(m.FHSrcId))
	m.FHDstMac = hex.EncodeToString(buf[8:16])
	m.FHSrcMac = hex.EncodeToString(buf[16:24])

	if m.DHRead {
		buf[offset] |= 0x80 // 1<<7
	}
	if m.DHAck {
		buf[offset] |= 0x40 // 1<<6
	}
	if m.DHDataFormat != 0 {
		buf[offset] |= (m.DHDataFormat & 0x3) << 4
	}
	if m.DHKeyLevel != 0 {
		buf[offset] |= (m.DHKeyLevel & 0x3) << 2
	}
	if m.DHEncryptType != 0 {
		buf[offset] |= m.DHEncryptType & 0x3
	}

	buf[offset+1] = m.DHDataSeq

	binary.LittleEndian.PutUint16(buf[offset+2:offset+4], m.DHMsgId)

	//需要crc检验的帧头有30字节

	copy(buf[offset+8:offset+16], m.DHSessionId)
	m.datacrc = append(m.datacrc, buf[offset+8:offset+16]...)
	m.datacrc = append(m.datacrc, m.Text...)
	switch m.DHKeyLevel {
	case 0:
		m.DHLength = uint16(len(m.Text))
		binary.LittleEndian.PutUint16(buf[offset+4:offset+6], uint16(len(m.Text)))
		buf[offset+6] = common.Crc(buf[:offset+6], offset+6)
		m.DHHeadCheck = buf[offset+6]
		buf[offset+7] = common.Crc(m.datacrc, len(m.datacrc))
		m.DHCheck = buf[offset+7]
		m.Final = append(m.Final, buf...)
		m.Final = append(m.Final, m.Text...)
	case 1:
		buf[offset+7] = common.Crc(m.datacrc, len(m.datacrc))
		m.DHCheck = buf[offset+7]
		key := RandomKeyGet(m.FHSequence, m.FHSrcId, m.FHTime)
		m.RK = key
		cipher, err := aes.Encrypt(m.datacrc, key)
		if err != nil {
			glog.Errorf("%v:%x", err, m.Final)
		}
		binary.LittleEndian.PutUint16(buf[offset+4:offset+6], uint16(len(cipher)-8))
		m.DHLength = uint16(len(cipher) - 8)
		buf[offset+6] = common.Crc(buf[:offset+6], offset+6)
		m.DHHeadCheck = buf[offset+6]
		m.Final = append(m.Final, buf[:offset+8]...)
		m.Final = append(m.Final, cipher...)
	case 2:
	case 3:
		if m.MZ != nil {
			buf[offset+7] = common.Crc(m.datacrc, len(m.datacrc))
			m.DHCheck = buf[offset+7]
			a := RandomKeyGet(m.FHSequence, m.FHSrcId, m.FHTime)
			m.RK = a
			key := make([]byte, 16)
			for n, x := range a {
				key[n] = x ^ m.MZ[n]
			}
			m.RKXORMZ = key
			if m.FHOpcode == 3 {
				guid, err := aes.Encrypt(m.FHGuid, key)
				if err != nil {
					glog.Errorf("%v,%v", err, m.Final)
				}
				if m.FHGuid != nil {
					copy(buf[FrameHeaderLen:offset], guid)
				}
			}
			cipher, err := aes.Encrypt(m.datacrc, key)
			if err != nil {
				glog.Errorf("%v,%v", err, m.Final)
			}
			m.CIPHER = cipher
			binary.LittleEndian.PutUint16(buf[offset+4:offset+6], uint16(len(cipher)-8))
			m.DHLength = uint16(len(cipher) - 8)
			buf[offset+6] = common.Crc(buf[:offset+6], offset+6)
			m.DHHeadCheck = buf[offset+6]
			m.Final = append(m.Final, buf[:offset+8]...)
			m.Final = append(m.Final, cipher...)

		} else {
			return fmt.Errorf("MZ is nil but DHKeyLevel is 3,%x", m.Final)
		}
	}
	if m.FHMask {
		switch m.FHOpcode {
		case 0x03:
			MaskContent(m.Final[8:40], m.Final[4:8])
		default:
			MaskContent(m.Final[8:32], m.Final[4:8])
		}
	}
	return nil
}

func RandomKeyGet(seq uint16, dst int64, time uint32) []byte {
	id := make([]uint8, 8)
	binary.LittleEndian.PutUint64(id, uint64(dst))
	key := make([]byte, 16)
	var timeArry [4]uint8
	var timeXor, i, n uint8

	binary.LittleEndian.PutUint32(timeArry[0:], time)
	timeXor = timeArry[0] ^ timeArry[1] ^ timeArry[2] ^ timeArry[3]

	binary.LittleEndian.PutUint32(key[0:], time)
	copy(key[4:], id[0:8])
	binary.LittleEndian.PutUint16(key[12:], seq)
	binary.LittleEndian.PutUint16(key[14:], seq)
	for i = 0; i < ((timeXor & 0x03) + 1); i++ {
		for n = 0; n < 16; n++ {
			key[n] ^= id[(timeXor+(i+n))&0x07] ^ timeArry[((i+n)&0x03)]
		}
	}
	return key
}
func MaskContent(payload []byte, mask []byte) {
	desp := mask[0]
	mask_index := byte(0)
	for iter := uint16(0); iter < uint16(len(payload)); iter++ {
		/* rotate mask and apply it */
		mask_index = byte((iter + uint16(desp)) % 4)
		payload[iter] ^= mask[mask_index]
	} /* end while */
}
