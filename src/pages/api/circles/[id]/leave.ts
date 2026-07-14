import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { CircleRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 退出圈子接口
 *
 * POST /api/circles/:id/leave
 * Headers: Authorization: Bearer <token>
 */

/** 退出圈子响应数据 */
type LeaveResult = {
  left: boolean
}

/**
 * 从请求中解析目标圈子 ID
 * @param req Next.js 请求对象
 * @returns 解析后的数字 ID 或 null
 */
function parseCircleId(req: NextApiRequest): number | null {
  const raw = Array.isArray(req.query.id) ? req.query.id[0] : req.query.id
  if (!raw) return null
  const id = Number(raw)
  return Number.isNaN(id) ? null : id
}

/**
 * 处理退出圈子请求
 * 校验圈子存在性与成员身份，拥有者不可退出。
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<LeaveResult>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const circleId = parseCircleId(req)
    if (!circleId) {
      return sendError(res, '无效的圈子 ID', 'INVALID_ID')
    }

    const circle = await CircleRepository.findById(circleId)
    if (!circle) {
      return sendNotFound(res, '圈子不存在')
    }

    // 校验当前用户是否为圈子成员
    const membership = await CircleRepository.findMembership(circleId, user.id)
    if (!membership) {
      return sendError(res, '您不是该圈子的成员', 'NOT_MEMBER', 409)
    }

    // 拥有者不可退出
    if (membership.role === 'OWNER') {
      return sendError(res, '圈子拥有者不能退出，请先转让圈子', 'OWNER_CANNOT_LEAVE', 400)
    }

    await CircleRepository.removeMember(circleId, user.id)
    await CircleRepository.incrementMemberCount(circleId, -1)

    return sendSuccess(res, { left: true }, '已退出圈子')
  } catch (error) {
    const message = error instanceof Error ? error.message : '退出圈子失败'
    return sendInternalError(res, message)
  }
}
