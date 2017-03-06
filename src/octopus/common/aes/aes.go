package aes

import (
	"bytes"
	"crypto/aes"
	"fmt"
)

func Encrypt(origData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	origData = PKCS5Padding(origData, block.BlockSize())
	crypted := make([]byte, len(origData))
	quotient := len(origData) / block.BlockSize()
	for i := 0; i < quotient; i++ {
		block.Encrypt(crypted[i*block.BlockSize():i*block.BlockSize()+block.BlockSize()], origData[i*block.BlockSize():i*block.BlockSize()+block.BlockSize()])
	}
	return crypted, nil
}
func DealingCipher(crypted, key []byte) ([]byte, error) {
	first := crypted[24]
	switch first & 0x03 {
	case 0:
		return crypted, nil
	case 1:
	case 2:
	case 3:
	}
	return Decrypt(crypted, key)
}
func Decrypt(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	origData := make([]byte, len(crypted))
	quotient := len(origData) / block.BlockSize()
	for i := 0; i < quotient; i++ {
		block.Decrypt(origData[i*block.BlockSize():i*block.BlockSize()+block.BlockSize()], crypted[i*block.BlockSize():i*block.BlockSize()+block.BlockSize()])
	}
	origData, err = PKCS5UnPadding(origData)
	if err != nil {
		return origData, err
	}
	return origData, nil
}

func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS5UnPadding(origData []byte) ([]byte, error) {
	length := len(origData)
	unpadding := int(origData[length-1])
	if length-unpadding <= 0 || unpadding > 0x10 {
		return nil, fmt.Errorf("PKCS5UnPadding err:%d<=%d,%x", length, unpadding, origData)
	}
	return origData[:(length - unpadding)], nil
}

func EncryptNp(origData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	crypted := make([]byte, len(origData))
	quotient := len(origData) / block.BlockSize()
	for i := 0; i < quotient; i++ {
		block.Encrypt(crypted[i*block.BlockSize():i*block.BlockSize()+block.BlockSize()], origData[i*block.BlockSize():i*block.BlockSize()+block.BlockSize()])
	}
	return crypted, nil
}
func DecryptNp(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	origData := make([]byte, len(crypted))
	quotient := len(origData) / block.BlockSize()
	for i := 0; i < quotient; i++ {
		block.Decrypt(origData[i*block.BlockSize():i*block.BlockSize()+block.BlockSize()], crypted[i*block.BlockSize():i*block.BlockSize()+block.BlockSize()])
	}
	return origData, nil
}
