import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser, toPublicUser } from '@/lib/auth-context'
import { MessageRepository, NotificationRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 当前用户信息响应数据
 */
type MeData = {
  user: ReturnType<typeof toPublicUser>
  hasPassword: boolean
  unreadMessages: number
  unreadNotifications: number
}

/**
 * 获取当前登录用户信息接口
 *
 * GET /api/auth/me
 * Headers: Authorization: Bearer <token>
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<MeData>>
) {
  if (req.method !== 'GET') {
    return sendError(res, '仅支持 GET 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const [unreadMessages, unreadNotifications] = await Promise.all([
      MessageRepository.countUnread(user.id),
      NotificationRepository.countUnread(user.id)
    ])

    return sendSuccess(res, {
      user: toPublicUser(user),
      hasPassword: !!user.password,
      unreadMessages,
      unreadNotifications
    })
  } catch (error) {
    const message = error instanceof Error ? error.message : '获取用户信息失败'
    return sendInternalError(res, message)
  }
}
