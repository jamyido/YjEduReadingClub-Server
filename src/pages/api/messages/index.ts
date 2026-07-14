import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { MessageRepository } from '@/db/repositories'
import {
  sendError,
  sendPaginated,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 私信列表接口
 *
 * GET /api/messages  分页查询当前用户的消息（需登录）
 * 查询参数：page、pageSize、onlyUnread
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  if (req.method !== 'GET') {
    return sendError(res, '仅支持 GET 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const page = Number(req.query.page) || 1
    const pageSize = Number(req.query.pageSize) || 20
    const onlyUnread =
      req.query.onlyUnread === 'true' || req.query.onlyUnread === '1'

    const { list, total } = await MessageRepository.findMany({
      userId: user.id,
      page,
      pageSize,
      onlyUnread
    })

    return sendPaginated(res, list, total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询消息列表失败'
    return sendInternalError(res, message)
  }
}
