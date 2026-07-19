// Package businessdate 提供 Asia/Shanghai 业务日期相关工具。
// 打卡连续天数的判断依赖本包，避免服务器时区影响业务日期。
package businessdate

import "time"

// CheckInTopicSlug 是系统默认打卡话题的稳定 slug。
const CheckInTopicSlug = "check-in-challenge"

// shanghaiOffset 是中国标准时间相对 UTC 的固定偏移（+8 小时）。
const shanghaiOffset = 8 * 60 * 60

// daySeconds 是一天的秒数。
const daySeconds = 24 * 60 * 60

// padTwo 将数字补齐为两位字符串。
func padTwo(value int) string {
	if value < 10 {
		return "0" + itoa(value)
	}
	return itoa(value)
}

// itoa 是轻量整数转字符串实现，避免引入 strconv 依赖。
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := false
	if value < 0 {
		negative = true
		value = -value
	}
	buf := [20]byte{}
	pos := len(buf)
	for value > 0 {
		pos--
		buf[pos] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// ToShanghaiDateKey 将任意时间转换为 Asia/Shanghai 的 YYYY-MM-DD 业务日期。
// 使用固定 UTC+8 偏移，不受进程或数据库服务器本地时区影响。
func ToShanghaiDateKey(t time.Time) string {
	shifted := t.UTC().Add(time.Duration(shanghaiOffset) * time.Second)
	return itoa(shifted.Year()) + "-" + padTwo(int(shifted.Month())) + "-" + padTwo(shifted.Day())
}

// GetPreviousShanghaiDateKey 获取指定时间前一天对应的北京时间业务日期。
func GetPreviousShanghaiDateKey(t time.Time) string {
	return ToShanghaiDateKey(t.Add(-time.Duration(daySeconds) * time.Second))
}

// GetEffectiveStreakDays 计算当前仍然有效的连续打卡天数。
// 最后打卡发生在今天或昨天时连续记录仍有效；更早则视为已经断签。
func GetEffectiveStreakDays(streakDays int, lastCheckInAt *time.Time, now time.Time) int {
	if streakDays <= 0 || lastCheckInAt == nil {
		return 0
	}
	lastDateKey := ToShanghaiDateKey(*lastCheckInAt)
	todayDateKey := ToShanghaiDateKey(now)
	yesterdayDateKey := GetPreviousShanghaiDateKey(now)
	if lastDateKey == todayDateKey || lastDateKey == yesterdayDateKey {
		return streakDays
	}
	return 0
}
