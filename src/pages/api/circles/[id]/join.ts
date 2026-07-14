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
 * 加入圈子接口
 *
 * POST /api/circles/:id/join
 * Headers: Authorization: Bearer <token>
 */

/** 加入圈子响应数据 */
type JoinResult = {
  joined: boolean
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
 * 处理加入圈子请求
 * 校验圈子存在性以及是否已是成员，避免重复加入。
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<JoinResult>>
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

    // 检查是否已是成员，避免重复加入
    const membership = await CircleRepository.findMembership(circleId, user.id)
    if (membership) {
      return sendError(res, '已经是该圈子的成员', 'ALREADY_MEMBER', 409)
    }

    await CircleRepository.addMember(circleId, user.id)
    await CircleRepository.incrementMemberCount(circleId, 1)

    return sendSuccess(res, { joined: true }, '加入圈子成功')
  } catch (error) {
    const message = error instanceof Error ? error.message : '加入圈子失败'
    return sendInternalError(res, message)
  }
}
