import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { MessageRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/** 全部已读响应数据 */
interface ReadAllResult {
  count: number
}

/**
 * 消息一键已读接口
 *
 * POST /api/messages/read-all  将当前用户所有未读消息标记为已读（需登录）
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<ReadAllResult>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const count = await MessageRepository.markAllRead(user.id)

    return sendSuccess(res, { count }, '全部已读')
  } catch (error) {
    const message = error instanceof Error ? error.message : '标记已读失败'
    return sendInternalError(res, message)
  }
}
