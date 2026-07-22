package utils

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword bcrypt 加密密码（新用户注册用）
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword 验证 bcrypt 密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// MD5Hash MD5哈希（兼容老系统密码格式：MD5(密码+盐)）
func MD5Hash(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

// RandStr 加密安全随机字符串（生成盐值/推广码）
var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func RandStr(n int) string {
	b := make([]rune, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[idx.Int64()]
	}
	return string(b)
}
