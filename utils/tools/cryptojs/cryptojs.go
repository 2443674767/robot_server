package cryptojs

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gctx"
)

var (
	aesEncryptKey, _ = gcfg.Instance("app").Get(gctx.New(), "app.aesEncryptKey") // 加密密钥，必须为16位
	iv, _            = gcfg.Instance("app").Get(gctx.New(), "app.aesIv")         // 偏移量，必须为16位
)

// AES加密
func AesEncrypt(content string) (string, error) {
	block, err := aes.NewCipher([]byte(aesEncryptKey.String()))
	if err != nil {
		return "", err
	}
	paddedPlaintext := PKCS7Padding([]byte(content), block.BlockSize())
	ciphertext := make([]byte, len(paddedPlaintext))
	mode := cipher.NewCBCEncrypter(block, []byte(iv.String()))
	mode.CryptBlocks(ciphertext, paddedPlaintext)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AES解密
func AesDecrypt(crypted string) (string, error) {
	cryptedBytes, err := base64.StdEncoding.DecodeString(crypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher([]byte(aesEncryptKey.String()))
	if err != nil {
		return "", err
	}
	blockMode := cipher.NewCBCDecrypter(block, []byte(iv.String()))
	origData := make([]byte, len(cryptedBytes))
	blockMode.CryptBlocks(origData, cryptedBytes)
	origData = PKCS7UnPadding(origData)
	return string(origData), nil
}

// PKCS7 Padding
func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// PKCS7 UnPadding
func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}
