import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { UserRepository, FollowRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 用户关注/取关接口
 *
 * POST   /api/users/:id/follow   关注该用户
 * DELETE /api/users/:id/follow   取消关注该用户
 * Headers: Authorization: Bearer <token>
 */

/** 关注操作响应数据 */
type FollowResult = {
  following: boolean
}

/**
 * 从请求中解析目标用户 ID
 * @param req Next.js 请求对象
 * @returns 解析后的数字 ID 或 null
 */
function parseTargetId(req: NextApiRequest): number | null {
  const raw = Array.isArray(req.query.id) ? req.query.id[0] : req.query.id
  if (!raw) return null
  const id = Number(raw)
  return Number.isNaN(id) ? null : id
}

/**
 * 处理关注与取消关注请求
 * 关注时会校验目标用户存在性、防止自我关注与重复关注；
 * 取关时会校验关注关系是否存在，并同步更新双方计数。
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<FollowResult>>
) {
  if (req.method !== 'POST' && req.method !== 'DELETE') {
    return sendError(res, '仅支持 POST 或 DELETE 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const currentUser = await getCurrentUser(req)
    if (!currentUser) {
      return sendUnauthorized(res)
    }

    const targetId = parseTargetId(req)
    if (!targetId) {
      return sendError(res, '无效的用户 ID', 'INVALID_ID')
    }

    // 不能关注自己
    if (currentUser.id === targetId) {
      return sendError(res, '不能关注自己', 'CANNOT_FOLLOW_SELF', 400)
    }

    // 校验目标用户存在
    const targetUser = await UserRepository.findById(targetId)
    if (!targetUser) {
      return sendNotFound(res, '用户不存在')
    }

    if (req.method === 'POST') {
      // 检查是否已关注，避免重复关注
      const existing = await FollowRepository.findRelation(currentUser.id, targetId)
      if (existing) {
        return sendError(res, '已经关注该用户', 'ALREADY_FOLLOWING', 409)
      }

      await FollowRepository.follow(currentUser.id, targetId)
      // 同步更新双方关注/粉丝计数
      await UserRepository.updateFollowCounts(currentUser.id, 1, 0)
      await UserRepository.updateFollowCounts(targetId, 0, 1)

      return sendSuccess(res, { following: true }, '关注成功')
    }

    // DELETE：取消关注
    const existing = await FollowRepository.findRelation(currentUser.id, targetId)
    if (!existing) {
      return sendError(res, '尚未关注该用户', 'NOT_FOLLOWING', 409)
    }

    await FollowRepository.unfollow(currentUser.id, targetId)
    await UserRepository.updateFollowCounts(currentUser.id, -1, 0)
    await UserRepository.updateFollowCounts(targetId, 0, -1)

    return sendSuccess(res, { following: false }, '已取消关注')
  } catch (error) {
    const message = error instanceof Error ? error.message : '关注操作失败'
    return sendInternalError(res, message)
  }
}
