// Package nickname 提供随机昵称生成能力。
// 用于微信登录静默注册等场景，避免空昵称。
package nickname

import (
	"math/rand"
	"time"
)

// adjectives 是随机昵称的形容词池。
var adjectives = []string{
	"热爱", "坚持", "专注", "阳光", "温柔", "快乐", "宁静", "勇敢",
	"智慧", "勤奋", "好奇", "开朗", "认真", "从容", "独立", "真诚",
}

// nouns 是随机昵称的名词池。
var nouns = []string{
	"读者", "书友", "学者", "行者", "探索者", "思考者", "记录者",
	"观察者", "追梦人", "读书郎", "阅己者", "知新者",
}

// randomSource 初始化时已播种的随机数生成器，避免每次调用重复播种。
var randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

// GenerateNickname 生成随机昵称。
// 格式：形容词 + 名词 + 4 位随机数字，如「热爱读者3827」。
func GenerateNickname() string {
	adj := adjectives[randomSource.Intn(len(adjectives))]
	noun := nouns[randomSource.Intn(len(nouns))]
	suffix := 1000 + randomSource.Intn(9000)
	return adj + noun + itoa(suffix)
}

// itoa 是轻量整数转字符串实现。
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	buf := [10]byte{}
	pos := len(buf)
	for value > 0 {
		pos--
		buf[pos] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[pos:])
}
