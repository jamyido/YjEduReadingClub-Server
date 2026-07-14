import type { NextApiRequest, NextApiResponse } from 'next'
import { UserRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import { getEffectiveStreakDays } from '@/lib/business-date'

/**
 * 用户公开资料接口
 *
 * GET /api/users/:id
 * 无需登录鉴权，返回用户的公开资料信息
 */

/** 用户公开资料响应数据 */
type UserProfile = {
  id: number
  nickname: string
  avatar: string | null
  bio: string | null
  gender: string
  streakDays: number
  followingCount: number
  followerCount: number
  createdAt: string
}

/**
 * 从请求中解析目标用户 ID
 * @param req Next.js 请求对象
 * @returns 解析后的数字 ID 或 null
 */
function parseUserId(req: NextApiRequest): number | null {
  const raw = Array.isArray(req.query.id) ? req.query.id[0] : req.query.id
  if (!raw) return null
  const id = Number(raw)
  return Number.isNaN(id) ? null : id
}

/**
 * 处理用户公开资料查询请求
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<UserProfile>>
) {
  if (req.method !== 'GET') {
    return sendError(res, '仅支持 GET 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const id = parseUserId(req)
    if (!id) {
      return sendError(res, '无效的用户 ID', 'INVALID_ID')
    }

    const user = await UserRepository.findById(id)
    if (!user) {
      return sendNotFound(res, '用户不存在')
    }

    const profile: UserProfile = {
      id: user.id,
      nickname: user.nickname,
      avatar: user.avatar,
      bio: user.bio,
      gender: user.gender,
      streakDays: getEffectiveStreakDays(user.streakDays, user.lastCheckInAt),
      followingCount: user.followingCount,
      followerCount: user.followerCount,
      createdAt: user.createdAt.toISOString()
    }

    return sendSuccess(res, profile)
  } catch (error) {
    const message = error instanceof Error ? error.message : '获取用户资料失败'
    return sendInternalError(res, message)
  }
}
