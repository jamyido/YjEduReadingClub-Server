// Package password 封装密码哈希、校验与强度校验工具。
// 使用 bcrypt 算法加盐哈希，保证存储安全。
package password

import (
	"errors"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// saltRounds 是 bcrypt 加盐轮数，10 是业界推荐值，兼顾安全与性能。
const saltRounds = 10

// HashPassword 对明文密码进行 bcrypt 哈希。
// 返回的字符串可直接持久化到数据库。
func HashPassword(plainPassword string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plainPassword), saltRounds)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// ComparePassword 校验明文密码与哈希密码是否匹配。
// 匹配返回 nil，不匹配返回错误。
func ComparePassword(plainPassword, hashedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

// ValidatePasswordStrength 校验密码强度。
// 规则：长度 6-32 位，至少包含字母和数字。
// 校验通过返回 nil，不通过返回具体错误说明。
func ValidatePasswordStrength(password string) error {
	if len(password) < 6 {
		return errors.New("密码长度至少 6 位")
	}
	if len(password) > 32 {
		return errors.New("密码长度不能超过 32 位")
	}
	hasLetter := false
	hasDigit := false
	for _, ch := range password {
		if unicode.IsLetter(ch) {
			hasLetter = true
		}
		if unicode.IsDigit(ch) {
			hasDigit = true
		}
	}
	if !hasLetter {
		return errors.New("密码需至少包含一个字母")
	}
	if !hasDigit {
		return errors.New("密码需至少包含一个数字")
	}
	return nil
}

// chinesePhoneRegex 匹配中国大陆手机号：1 开头、第二位 3-9、共 11 位。
var chinesePhoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

// IsValidChinesePhone 校验中国大陆手机号格式。
func IsValidChinesePhone(phone string) bool {
	return chinesePhoneRegex.MatchString(strings.TrimSpace(phone))
}
