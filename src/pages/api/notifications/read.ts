import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { NotificationRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendNotFound,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/** 标记已读请求体 */
interface MarkReadBody {
  id?: number
}

/**
 * 通知已读接口
 *
 * POST /api/notifications/read  标记单条或全部通知为已读（需登录）
 * Body: { id? }  不传 id 时标记全部已读
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const body = (req.body || {}) as MarkReadBody
    const hasId = Object.prototype.hasOwnProperty.call(body, 'id')
    const id = typeof body.id === 'number' && Number.isInteger(body.id) && body.id > 0
      ? body.id
      : null

    if (hasId && !id) {
      return sendError(res, '通知 ID 必须是正整数', 'INVALID_NOTIFICATION_ID')
    }

    // 传 id：标记单条通知为已读
    if (id) {
      const updated = await NotificationRepository.markRead(id, user.id)
      if (!updated) {
        return sendNotFound(res, '通知不存在或无权操作')
      }
      return sendSuccess(res, updated, '已标记为已读')
    }

    // 未传 id：标记全部通知为已读
    const count = await NotificationRepository.markAllRead(user.id)
    return sendSuccess(res, { count }, '全部已读')
  } catch (error) {
    const message = error instanceof Error ? error.message : '标记已读失败'
    return sendInternalError(res, message)
  }
}
