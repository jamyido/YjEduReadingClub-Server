/** 中国标准时间相对 UTC 的固定偏移量。 */
const SHANGHAI_OFFSET_MS = 8 * 60 * 60 * 1000

/** 一天的毫秒数。 */
const DAY_MS = 24 * 60 * 60 * 1000

/** 系统默认打卡话题的稳定 slug。 */
export const CHECK_IN_TOPIC_SLUG = 'check-in-challenge'

/**
 * 将数字补齐为两位字符串。
 * @param value 月或日数值
 * @returns 两位十进制字符串
 */
function padTwo(value: number): string {
  return value < 10 ? '0' + String(value) : String(value)
}

/**
 * 将任意时间转换为 Asia/Shanghai 的 YYYY-MM-DD 业务日期。
 * 中国标准时间全年固定为 UTC+8，因此使用 UTC 字段读取偏移后的时间，
 * 不受 Node 进程或数据库服务器本地时区影响。
 * @param date 待转换时间
 * @returns 北京时间业务日期
 */
export function toShanghaiDateKey(date: Date = new Date()): string {
  const shifted = new Date(date.getTime() + SHANGHAI_OFFSET_MS)
  return String(shifted.getUTCFullYear())
    + '-'
    + padTwo(shifted.getUTCMonth() + 1)
    + '-'
    + padTwo(shifted.getUTCDate())
}

/**
 * 获取指定时间前一天对应的北京时间业务日期。
 * @param date 参照时间
 * @returns 前一天的 YYYY-MM-DD 日期
 */
export function getPreviousShanghaiDateKey(date: Date = new Date()): string {
  return toShanghaiDateKey(new Date(date.getTime() - DAY_MS))
}

/**
 * 计算当前仍然有效的连续打卡天数。
 * 最后打卡发生在今天或昨天时连续记录仍有效；更早则视为已经断签。
 * @param streakDays 数据库保存的连续天数
 * @param lastCheckInAt 最后打卡时间
 * @param now 当前时间，测试时可传入固定值
 * @returns 对外展示的有效连续天数
 */
export function getEffectiveStreakDays(
  streakDays: number,
  lastCheckInAt: Date | null,
  now: Date = new Date()
): number {
  if (streakDays <= 0 || !lastCheckInAt) {
    return 0
  }

  const lastDateKey = toShanghaiDateKey(lastCheckInAt)
  const todayDateKey = toShanghaiDateKey(now)
  const yesterdayDateKey = getPreviousShanghaiDateKey(now)

  return lastDateKey === todayDateKey || lastDateKey === yesterdayDateKey
    ? streakDays
    : 0
}
