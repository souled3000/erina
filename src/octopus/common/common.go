package common

func Crc(msg []byte, n int) byte {
	var headerCheck uint8
	for i := 0; i < n; i++ {
		headerCheck ^= msg[i]
		for bit := 8; bit > 0; bit-- {
			if headerCheck&0x80 != 0 {
				headerCheck = (headerCheck << 1) ^ 0x31
			} else {
				headerCheck = (headerCheck << 1)
			}
		}
	}
	return headerCheck
}