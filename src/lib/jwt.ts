import jwt from 'jsonwebtoken'

/**
 * JWT Token 载荷结构
 */
export interface JwtPayload {
  userId: number
  phone: string
  role: string
}

/**
 * 临时登录 Token 载荷结构（用于微信登录中间态）
 * 在微信登录后、获取手机号前，携带 openid 与 session_key
 */
export interface WeappTempPayload {
  openid: string
  sessionKey: string
  unionid?: string
}

/**
 * 获取 JWT 签名密钥
 * @returns 环境变量中配置的 JWT_SECRET
 */
function getSecret(): string {
  const secret = process.env.JWT_SECRET
  if (!secret) {
    throw new Error('JWT_SECRET 环境变量未配置')
  }
  return secret
}

/**
 * 签发用户认证 Token
 * @param payload 用户信息载荷
 * @returns 签名后的 JWT 字符串
 */
export function signToken(payload: JwtPayload): string {
  const secret = getSecret()
  const expiresIn = process.env.JWT_ACCESS_TOKEN_EXPIRES_IN || '7d'
  return jwt.sign(payload, secret, { expiresIn } as jwt.SignOptions)
}

/**
 * 签发微信登录临时 Token
 * 有效期较短（10 分钟），仅用于获取手机号前的中间态
 * @param payload 微信临时载荷
 * @returns 签名后的临时 JWT 字符串
 */
export function signWeappTempToken(payload: WeappTempPayload): string {
  const secret = getSecret()
  return jwt.sign(payload, secret, { expiresIn: '10m' })
}

/**
 * 验证并解析用户认证 Token
 * @param token JWT 字符串
 * @returns 解析后的载荷，验证失败返回 null
 */
export function verifyToken(token: string): JwtPayload | null {
  try {
    const secret = getSecret()
    const decoded = jwt.verify(token, secret) as JwtPayload
    return decoded
  } catch {
    return null
  }
}

/**
 * 验证并解析微信临时 Token
 * @param token 临时 JWT 字符串
 * @returns 解析后的载荷，验证失败返回 null
 */
export function verifyWeappTempToken(token: string): WeappTempPayload | null {
  try {
    const secret = getSecret()
    const decoded = jwt.verify(token, secret) as WeappTempPayload
    return decoded
  } catch {
    return null
  }
}

/**
 * 从 Authorization 头中提取 Bearer Token
 * @param authHeader Authorization 头的值
 * @returns 提取出的 token，格式不正确返回 null
 */
export function extractBearerToken(authHeader: string | undefined | null): string | null {
  if (!authHeader) return null
  const parts = authHeader.split(' ')
  if (parts.length === 2 && parts[0] === 'Bearer') {
    return parts[1]
  }
  return null
}
