import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { MessageRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendPaginated,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import type { Message } from '@prisma/client'

/** 发送消息请求体 */
interface SendMessageBody {
  content?: string
}

/**
 * 解析动态路由参数中的用户 ID
 * @param query 路由查询参数（string | string[] | undefined）
 * @returns 解析出的正整数 ID 或 null
 */
function parseUserIdParam(
  query: string | string[] | undefined
): number | null {
  if (!query) return null
  const value = Array.isArray(query) ? query[0] : query
  const id = Number(value)
  return Number.isFinite(id) && id > 0 ? id : null
}

/**
 * 与指定用户的会话接口入口
 *
 * GET  /api/messages/[userId]   获取与某用户的私聊记录，并标记对方消息为已读
 * POST /api/messages/[userId]   向某用户发送私信
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  const otherUserId = parseUserIdParam(req.query.userId)
  if (!otherUserId) {
    return sendError(res, '用户 ID 不合法', 'INVALID_USER_ID', 400)
  }

  switch (req.method) {
    case 'GET':
      return handleGetConversation(req, res, otherUserId)
    case 'POST':
      return handleSendMessage(req, res, otherUserId)
    default:
      return sendError(res, '仅支持 GET / POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }
}

/**
 * 获取当前用户与目标用户的私聊记录
 * 拉取后将对方发来的未读消息标记为已读
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 * @param otherUserId 对方用户 ID
 */
async function handleGetConversation(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>,
  otherUserId: number
) {
  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    if (otherUserId === user.id) {
      return sendError(res, '不能与自己发起会话', 'INVALID_USER_ID', 400)
    }

    const page = Number(req.query.page) || 1
    const pageSize = Number(req.query.pageSize) || 50

    const { list, total } = await MessageRepository.findConversation(
      user.id,
      otherUserId,
      page,
      pageSize
    )

    // 拉取后将对方发来的消息标记为已读
    await MessageRepository.markConversationRead(user.id, otherUserId)

    return sendPaginated(res, list, total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询会话失败'
    return sendInternalError(res, message)
  }
}

/**
 * 向目标用户发送一条私信
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 * @param otherUserId 对方用户 ID
 */
async function handleSendMessage(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<Message>>,
  otherUserId: number
) {
  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    if (otherUserId === user.id) {
      return sendError(res, '不能给自己发送私信', 'INVALID_USER_ID', 400)
    }

    const body = (req.body || {}) as SendMessageBody
    const content =
      typeof body.content === 'string' ? body.content.trim() : ''

    if (!content) {
      return sendError(res, '消息内容不能为空', 'EMPTY_CONTENT', 400)
    }

    const message = await MessageRepository.send(
      user.id,
      otherUserId,
      content
    )

    return sendSuccess(res, message, '发送成功', 201)
  } catch (error) {
    const message = error instanceof Error ? error.message : '发送消息失败'
    return sendInternalError(res, message)
  }
}
