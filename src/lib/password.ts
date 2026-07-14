import bcrypt from 'bcryptjs'

/**
 * bcrypt 加盐轮数
 * 值越高越安全但越慢，10 是业界推荐值
 */
const SALT_ROUNUNDS = 10

/**
 * 对明文密码进行 bcrypt 哈希
 * @param plainPassword 用户输入的明文密码
 * @returns 哈希后的密码字符串
 */
export function hashPassword(plainPassword: string): Promise<string> {
  return bcrypt.hash(plainPassword, SALT_ROUNUNDS)
}

/**
 * 验证明文密码与哈希密码是否匹配
 * @param plainPassword 用户输入的明文密码
 * @param hashedPassword 数据库中存储的哈希密码
 * @returns 匹配返回 true，不匹配返回 false
 */
export function comparePassword(
  plainPassword: string,
  hashedPassword: string
): Promise<boolean> {
  return bcrypt.compare(plainPassword, hashedPassword)
}

/**
 * 校验密码强度
 * 规则：长度 6-32 位，至少包含字母和数字
 * @param password 用户输入的明文密码
 * @returns 校验通过返回 null，不通过返回错误说明
 */
export function validatePasswordStrength(password: string): string | null {
  if (!password || password.length < 6) {
    return '密码长度至少 6 位'
  }
  if (password.length > 32) {
    return '密码长度不能超过 32 位'
  }
  if (!/[a-zA-Z]/.test(password)) {
    return '密码需至少包含一个字母'
  }
  if (!/[0-9]/.test(password)) {
    return '密码需至少包含一个数字'
  }
  return null
}

/**
 * 校验中国大陆手机号格式
 * @param phone 手机号字符串
 * @returns 格式正确返回 true
 */
export function isValidChinesePhone(phone: string): boolean {
  return /^1[3-9]\d{9}$/.test(phone)
}
