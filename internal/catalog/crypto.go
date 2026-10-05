package catalog

import (
	"bytes"
	"crypto/des"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// The APK obtains this fixed eight-byte DES key from its native library.
const directoryKey = "dw!@#$%^"

func decodeDirectory(encrypted []byte) ([]Server, error) {
	encrypted = bytes.TrimSpace(encrypted)
	if len(encrypted) == 0 || len(encrypted)%2 != 0 {
		return nil, errors.New("节点目录密文格式无效")
	}
	data, err := hex.DecodeString(string(encrypted))
	if err != nil || len(data) == 0 || len(data)%des.BlockSize != 0 {
		return nil, errors.New("节点目录密文格式无效")
	}
	cipher, err := des.NewCipher([]byte(directoryKey))
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(data); i += des.BlockSize {
		cipher.Decrypt(data[i:i+des.BlockSize], data[i:i+des.BlockSize])
	}
	padding := int(data[len(data)-1])
	if padding < 1 || padding > 8 || len(data) < padding {
		return nil, errors.New("节点目录填充无效")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, errors.New("节点目录填充无效")
		}
	}
	data = data[:len(data)-padding]
	// DesUtil additionally removes an inner padding value in [1,7].
	if len(data) > 0 {
		padding = int(data[len(data)-1])
		if padding > 0 && padding < 8 {
			if padding > len(data) {
				return nil, errors.New("节点目录填充无效")
			}
			for _, b := range data[len(data)-padding:] {
				if int(b) != padding {
					return nil, errors.New("节点目录填充无效")
				}
			}
			data = data[:len(data)-padding]
		}
	}
	// Android JSONTokener accepts trailing characters <= ASCII space.
	data = bytes.TrimRightFunc(data, func(r rune) bool { return r <= 32 })
	var servers []Server
	if err = json.Unmarshal(data, &servers); err != nil || len(servers) == 0 || len(servers) > 10000 {
		return nil, errors.New("节点目录内容无效")
	}
	for i := range servers {
		s := &servers[i]
		if s.ID == "" || s.Name == "" {
			return nil, errors.New("节点目录字段无效")
		}
		if _, err = s.Address(); err != nil {
			return nil, fmt.Errorf("节点目录地址无效: %s", s.ID)
		}
		if strings.TrimSpace(s.Province) == "" {
			s.Province = "其他"
		}
	}
	return servers, nil
}
