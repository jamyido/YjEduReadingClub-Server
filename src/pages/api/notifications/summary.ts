import type { NextApiRequest, NextApiResponse } from 'next'
import { NotificationType } from '@prisma/client'
import { getCurrentUser } from '@/lib/auth-context'
import { NotificationRepository } from '@/db/repositories'
import {
  sendError,
  sendSuccess,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/** 消息页所需的通知总量与分类未读统计。 */
interface NotificationSummaryData {
  allTotal: number
  unreadTotal: number
  like: number
  reply: number
  system: number
  task: number
}

/**
 * 通知汇总接口。
 * GET /api/notifications/summary 返回当前用户的通知总数和分类未读数。
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<NotificationSummaryData>>
) {
  if (req.method !== 'GET') {
    return sendError(res, '仅支持 GET 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const [allTotal, typeCounts] = await Promise.all([
      NotificationRepository.countAll(user.id),
      NotificationRepository.countUnreadByType(user.id)
    ])
    const summary: NotificationSummaryData = {
      allTotal,
      unreadTotal: 0,
      like: 0,
      reply: 0,
      system: 0,
      task: 0
    }

    typeCounts.forEach(function (item) {
      const targetType = item.targetType ? item.targetType.toLowerCase() : ''
      summary.unreadTotal += item.count
      if (item.type === NotificationType.LIKE) {
        summary.like += item.count
      } else if (item.type === NotificationType.COMMENT) {
        summary.reply += item.count
      } else if (
        item.type === NotificationType.TASK
        || (item.type === NotificationType.SYSTEM && targetType === 'task')
      ) {
        summary.task += item.count
      } else {
        summary.system += item.count
      }
    })

    return sendSuccess(res, summary)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询通知汇总失败'
    return sendInternalError(res, message)
  }
}
