import type { User } from '@prisma/client'
import type { NextApiRequest } from 'next'
import { UserRepository } from '@/db/repositories'
import { verifyToken, extractBearerToken } from './jwt'
import { getEffectiveStreakDays } from './business-date'

/**
 * 认证上下文：从请求中提取并验证当前登录用户
 *
 * 职责：
 * - 从 Authorization 头中解析 Bearer Token
 * - 验证 Token 有效性并查询用户是否存在
 * - 向 API 路由提供统一的「获取当前用户」能力
 */

/**
 * 过滤后的公开用户信息（不含密码、openid 等敏感字段）
 */
export type PublicUser = Omit<User, 'password' | 'weappOpenId' | 'unionId'>

/**
 * 从请求头中解析出 userId
 * @param req Next.js 请求对象
 * @returns userId 或 null（未认证）
 */
export function getUserIdFromRequest(req: NextApiRequest): number | null {
  const token = extractBearerToken(req.headers.authorization)
  if (!token) return null
  const payload = verifyToken(token)
  if (!payload) return null
  return payload.userId
}

/**
 * 从请求中获取当前登录用户完整记录
 * @param req Next.js 请求对象
 * @returns 用户记录或 null（未认证或用户不存在）
 */
export async function getCurrentUser(req: NextApiRequest): Promise<User | null> {
  const userId = getUserIdFromRequest(req)
  if (!userId) return null
  const user = await UserRepository.findById(userId)
  if (!user) return null
  if (user.status === 'BANNED') return null
  return user
}

/**
 * 移除用户记录中的敏感字段，生成公开用户信息
 * @param user 用户完整记录
 * @returns 过滤后的公开用户信息
 */
export function toPublicUser(user: User): PublicUser {
  const { password, weappOpenId, unionId, ...publicFields } = user
  return {
    ...publicFields,
    streakDays: getEffectiveStreakDays(user.streakDays, user.lastCheckInAt)
  }
}

/**
 * 判断当前用户是否为管理员
 * @param user 用户记录
 * @returns 是管理员返回 true
 */
export function isAdmin(user: User | null): boolean {
  return user?.role === 'ADMIN'
}
