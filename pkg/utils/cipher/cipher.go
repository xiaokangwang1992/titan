/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/14 14/14/10
 @Desc    : aes cbc 加密解密实现
*/

package cipher

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

var (
	AES_IV = getEnv("AES_IV", "3010201735544643")
)

// Encrypt data to string
func Encrypt(data []byte, key string) (string, error) {
	// create cipher.Block
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	// padding content, if less than 16 characters
	blockSize := block.BlockSize()
	originData := pad(data, blockSize)
	// encryption method
	blockMode := cipher.NewCBCEncrypter(block, []byte(AES_IV))
	// encrypt, output to []byte array
	crypted := make([]byte, len(originData))
	blockMode.CryptBlocks(crypted, originData)
	// use StdEncoding and remove possible newline characters
	result := base64.StdEncoding.EncodeToString(crypted)
	return strings.ReplaceAll(result, "\n", ""), nil
}

// EncryptCompact encrypt data to compact base64URL encoded string (no padding characters)
func EncryptCompact(data []byte, key string) (string, error) {
	// create cipher.Block
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	// padding content, if less than 16 characters
	blockSize := block.BlockSize()
	originData := pad(data, blockSize)
	// encryption method
	blockMode := cipher.NewCBCEncrypter(block, []byte(AES_IV))
	// encrypt, output to []byte array
	crypted := make([]byte, len(originData))
	blockMode.CryptBlocks(crypted, originData)
	// use RawURLEncoding to ensure no padding characters and newline characters
	return base64.RawURLEncoding.EncodeToString(crypted), nil
}

func Decrypt(decryptText, key string) ([]byte, error) {
	decodeData, err := base64.StdEncoding.DecodeString(decryptText)
	if err != nil || len(decodeData) == 0 {
		return nil, fmt.Errorf("decrypt text is error")
	}
	// create cipher.Block
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	// decryption mode
	blockMode := cipher.NewCBCDecrypter(block, []byte(AES_IV))
	// output to []byte array
	originData := make([]byte, len(decodeData))
	blockMode.CryptBlocks(originData, decodeData)
	// remove padding, and return
	return unPad(originData), nil
}

// DecryptCompact decrypt data encrypted by EncryptCompact
func DecryptCompact(decryptText, key string) ([]byte, error) {
	decodeData, err := base64.RawURLEncoding.DecodeString(decryptText)
	if err != nil || len(decodeData) == 0 {
		return nil, fmt.Errorf("decrypt text is error")
	}
	// create cipher.Block
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	// decryption mode
	blockMode := cipher.NewCBCDecrypter(block, []byte(AES_IV))
	// output to []byte array
	originData := make([]byte, len(decodeData))
	blockMode.CryptBlocks(originData, decodeData)
	// remove padding, and return
	return unPad(originData), nil
}

func CheckEncrypt(cipherText string, key string) bool {
	decryptText, err := Decrypt(cipherText, key)
	if err != nil {
		return false
	}
	return decryptText != nil
}

func CheckEncryptCompact(cipherText string, key string) bool {
	decryptText, err := DecryptCompact(cipherText, key)
	if err != nil {
		return false
	}
	return decryptText != nil
}

func pad(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, padText...)
}

func unPad(cipherText []byte) []byte {
	length := len(cipherText)
	// remove the last padding
	unPadding := int(cipherText[length-1])
	return cipherText[:(length - unPadding)]
}

func getEnv(key, value string) string {
	if env := os.Getenv(key); env != "" {
		return env
	}
	return value
}
