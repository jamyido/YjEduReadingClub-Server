import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { NotificationRepository } from '@/db/repositories'
import type { NotificationListCategory } from '@/db/repositories'
import {
  sendError,
  sendPaginated,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/** 前端查询参数到仓库通知分类的白名单映射。 */
const categoryMap: Record<string, NotificationListCategory> = {
  like: 'LIKE',
  reply: 'REPLY',
  system: 'SYSTEM',
  task: 'TASK'
}

/**
 * 解析并限制通知列表分页参数。
 * @param req Next.js 请求对象
 * @returns 安全的分页参数
 */
function parsePagination(req: NextApiRequest): { page: number; pageSize: number } {
  const rawPage = Array.isArray(req.query.page) ? req.query.page[0] : req.query.page
  const rawPageSize = Array.isArray(req.query.pageSize)
    ? req.query.pageSize[0]
    : req.query.pageSize
  const parsedPage = Number(rawPage)
  const parsedPageSize = Number(rawPageSize)
  const page = Number.isInteger(parsedPage) && parsedPage > 0 ? parsedPage : 1
  const pageSize = Number.isInteger(parsedPageSize) && parsedPageSize > 0
    ? Math.min(100, parsedPageSize)
    : 20
  return { page, pageSize }
}

/**
 * 解析通知产品分类，未知值按未指定处理。
 * @param req Next.js 请求对象
 * @returns 仓库分类或 undefined
 */
function parseCategory(req: NextApiRequest): NotificationListCategory | undefined {
  const rawCategory = Array.isArray(req.query.category)
    ? req.query.category[0]
    : req.query.category
  if (!rawCategory) {
    return undefined
  }
  return categoryMap[String(rawCategory).toLowerCase()]
}

/**
 * 通知列表接口
 *
 * GET /api/notifications  分页查询当前用户的通知（需登录）
 * 查询参数：page、pageSize、onlyUnread、category(like/reply/system/task)
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

    const { page, pageSize } = parsePagination(req)
    const onlyUnread =
      req.query.onlyUnread === 'true' || req.query.onlyUnread === '1'
    const category = parseCategory(req)

    const { list, total } = await NotificationRepository.findMany(
      user.id,
      { page, pageSize, onlyUnread, category }
    )

    return sendPaginated(res, list, total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询通知列表失败'
    return sendInternalError(res, message)
  }
}
