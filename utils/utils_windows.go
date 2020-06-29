package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/tidwall/gjson"
)

const (
	chromeDir        = "/AppData/Local/Google/Chrome/User Data/*/"
	chromeKeyFile    = "/AppData/Local/Google/Chrome/User Data/Local State"
	edgeDir          = "/AppData/Local/Microsoft/Edge/User Data/*/"
	edgeKeyFile      = "/AppData/Local/Microsoft/Edge/User Data/Local State"
	speed360Dir      = "/AppData/Local/360chrome/Chrome/User Data/*/"
	speed360KeyFile  = ""
	qqBrowserDir     = "/AppData/Local/Tencent/QQBrowser/User Data/*/"
	qqBrowserKeyFile = ""
)

var (
	chromeKey []byte

	browserList = map[string]struct {
		Dir string
		Key string
	}{
		"chrome": {
			chromeDir,
			chromeKeyFile,
		},
		"edge": {
			edgeDir,
			edgeKeyFile,
		},
		"360speed": {
			speed360Dir,
			speed360KeyFile,
		},
		"qq": {
			qqBrowserDir,
			qqBrowserKeyFile,
		},
	}
)

func PickBrowser(name string) (browserDir, key string, err error) {
	name = strings.ToLower(name)
	if choice, ok := browserList[name]; ok {
		if choice.Key != "" {
			return os.Getenv("USERPROFILE") + choice.Dir, os.Getenv("USERPROFILE") + choice.Key, nil
		} else {
			return os.Getenv("USERPROFILE") + choice.Dir, "", nil
		}
	}
	return "", "", errBrowserNotSupported
}

var (
	errBase64DecodeFailed = errors.New("decode base64 failed")
)

func InitKey(key string) error {
	if key == "" {
		VersionUnder80 = true
		return nil
	}
	keyFile, err := ReadFile(key)
	if err != nil {
		return err
	}
	encryptedKey := gjson.Get(keyFile, "os_crypt.encrypted_key")
	if encryptedKey.Exists() {
		pureKey, err := base64.StdEncoding.DecodeString(encryptedKey.String())
		if err != nil {
			return errBase64DecodeFailed
		}
		chromeKey, err = decryptStringWithDPAPI(pureKey[5:])
		return nil
	} else {
		VersionUnder80 = true
		return nil
	}
}

func DecryptChromePass(encryptPass []byte) (string, error) {
	if len(encryptPass) > 15 {
		// remove prefix 'v10'
		return aesGCMDecrypt(encryptPass[15:], chromeKey, encryptPass[3:15])
	} else {
		return "", passwordIsEmpty
	}
}

// chromium > 80 https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_win.cc
func aesGCMDecrypt(crypted, key, nounce []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	blockMode, err := cipher.NewGCM(block)
	origData, err := blockMode.Open(nil, nounce, crypted, nil)
	if err != nil {
		return "", err
	}
	return string(origData), nil
}

type DataBlob struct {
	cbData uint32
	pbData *byte
}

func NewBlob(d []byte) *DataBlob {
	if len(d) == 0 {
		return &DataBlob{}
	}
	return &DataBlob{
		pbData: &d[0],
		cbData: uint32(len(d)),
	}
}

func (b *DataBlob) ToByteArray() []byte {
	d := make([]byte, b.cbData)
	copy(d, (*[1 << 30]byte)(unsafe.Pointer(b.pbData))[:])
	return d
}

// chrome < 80 https://chromium.googlesource.com/chromium/src/+/76f496a7235c3432983421402951d73905c8be96/components/os_crypt/os_crypt_win.cc#82
func decryptStringWithDPAPI(data []byte) ([]byte, error) {
	dllCrypt := syscall.NewLazyDLL("Crypt32.dll")
	dllKernel := syscall.NewLazyDLL("Kernel32.dll")
	procDecryptData := dllCrypt.NewProc("CryptUnprotectData")
	procLocalFree := dllKernel.NewProc("LocalFree")
	var outBlob DataBlob
	r, _, err := procDecryptData.Call(uintptr(unsafe.Pointer(NewBlob(data))), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&outBlob)))
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	return outBlob.ToByteArray(), nil
}

func DecryptStringWithDPAPI(data []byte) (string, error) {
	dllCrypt := syscall.NewLazyDLL("Crypt32.dll")
	dllKernel := syscall.NewLazyDLL("Kernel32.dll")
	procDecryptData := dllCrypt.NewProc("CryptUnprotectData")
	procLocalFree := dllKernel.NewProc("LocalFree")
	var outBlob DataBlob
	r, _, err := procDecryptData.Call(uintptr(unsafe.Pointer(NewBlob(data))), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&outBlob)))
	if r == 0 {
		return "", err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	return string(outBlob.ToByteArray()), nil
}