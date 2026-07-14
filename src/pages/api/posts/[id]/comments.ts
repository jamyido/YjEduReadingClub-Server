import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import {
  PostRepository,
  CommentRepository,
  UserRepository,
  NotificationRepository
} from '@/db/repositories'
import {
  sendSuccess,
  sendPaginated,
  sendError,
  sendUnauthorized,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import { NotificationType } from '@prisma/client'

/**
 * 从查询参数中解析帖子 ID
 * @param value 原始查询参数（可能为数组）
 * @returns 帖子 ID 或 null（格式不正确）
 */
function parsePostId(value: string | string[] | undefined): number | null {
  const raw = Array.isArray(value) ? value[0] : value
  if (!raw) return null
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

/**
 * 将查询字符串参数解析为正整数
 * @param value 原始查询字符串（可能为数组）
 * @param fallback 解析失败或为空时的默认值
 * @returns 解析后的正整数
 */
function parsePositiveInt(
  value: string | string[] | undefined,
  fallback: number,
  max?: number
): number {
  const raw = Array.isArray(value) ? value[0] : value
  if (!raw) return fallback
  const parsed = Number(raw)
  if (!Number.isInteger(parsed) || parsed <= 0) {
    return fallback
  }
  return max ? Math.min(max, parsed) : parsed
}

/** 创建评论请求体。 */
interface CreateCommentBody {
  content?: unknown
  parentId?: unknown
  replyToId?: unknown
}

/**
 * 将可选的 body 字段解析为正整数。
 * @param value 原始值（可能为 number / string / undefined）
 * @returns 未传时返回 undefined，合法时返回数字，格式错误时返回 null
 */
function parseOptionalPositiveInteger(value: unknown): number | null | undefined {
  if (value == null || value === '') return undefined
  const num = Number(value)
  return Number.isInteger(num) && num > 0 ? num : null
}

/**
 * 帖子评论列表与创建接口
 *
 * GET  /api/posts/:id/comments   分页查询帖子一级评论
 * POST /api/posts/:id/comments   创建评论（需登录）
 *
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  const id = parsePostId(req.query.id)
  if (id === null) {
    return sendError(res, '帖子 ID 格式不正确', 'INVALID_ID')
  }

  if (req.method === 'GET') {
    return handleList(req, res, id)
  }
  if (req.method === 'POST') {
    return handleCreate(req, res, id)
  }
  return sendError(res, '仅支持 GET/POST 请求', 'METHOD_NOT_ALLOWED', 405)
}

/**
 * 分页查询帖子评论列表
 * @param req 请求对象，读取 page / pageSize 查询参数
 * @param res 响应对象
 * @param postId 帖子 ID
 */
async function handleList(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>,
  postId: number
) {
  try {
    const post = await PostRepository.findById(postId)
    if (!post || post.status === 1) {
      return sendNotFound(res, '帖子不存在或已删除')
    }

    const page = parsePositiveInt(req.query.page, 1)
    const pageSize = parsePositiveInt(req.query.pageSize, 20, 100)

    const { list, total } = await CommentRepository.findByPost(postId, page, pageSize)
    return sendPaginated(res, list, total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询评论列表失败'
    return sendInternalError(res, message)
  }
}

/**
 * 创建评论
 *
 * 流程：
 * 1. 校验登录态、帖子存在性与正文内容
 * 2. 写入评论（含父评论 / 回复目标）
 * 3. 帖子评论数 +1
 * 4. 通知接收方：回复时通知线程中的目标用户，否则通知帖子作者；自己回复自己不通知
 *
 * @param req 请求对象，Body: { content, parentId?, replyToId? }
 * @param res 响应对象
 * @param postId 帖子 ID
 */
async function handleCreate(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>,
  postId: number
) {
  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const post = await PostRepository.findById(postId)
    if (!post || post.status === 1) {
      return sendNotFound(res, '帖子不存在或已删除')
    }

    const body = req.body && typeof req.body === 'object'
      ? req.body as CreateCommentBody
      : {}
    const { content, parentId, replyToId } = body
    if (!content || typeof content !== 'string' || content.trim().length === 0) {
      return sendError(res, '评论内容不能为空', 'INVALID_CONTENT')
    }

    const parsedParentId = parseOptionalPositiveInteger(parentId)
    const parsedReplyToId = parseOptionalPositiveInteger(replyToId)

    if (parsedParentId === null) {
      return sendError(res, '父评论 ID 必须是正整数', 'INVALID_PARENT_COMMENT')
    }
    if (parsedReplyToId === null) {
      return sendError(res, '被回复用户 ID 必须是正整数', 'INVALID_REPLY_USER')
    }

    if (parsedReplyToId && !parsedParentId) {
      return sendError(res, '回复用户时必须指定父评论', 'INVALID_REPLY_TARGET')
    }

    const parentComment = parsedParentId
      ? await CommentRepository.findById(parsedParentId)
      : null
    if (
      parsedParentId
      && (
        !parentComment
        || parentComment.postId !== postId
        || parentComment.status !== 0
        || parentComment.parentId !== null
      )
    ) {
      return sendError(res, '父评论不存在或不属于当前帖子', 'INVALID_PARENT_COMMENT')
    }

    if (parsedReplyToId) {
      const [replyUser, isThreadParticipant] = await Promise.all([
        UserRepository.findById(parsedReplyToId),
        CommentRepository.isActiveThreadParticipant(parsedParentId || 0, parsedReplyToId)
      ])
      if (!replyUser || replyUser.status !== 'ACTIVE' || !isThreadParticipant) {
        return sendError(res, '被回复用户不存在或不属于当前评论线程', 'INVALID_REPLY_USER')
      }
    }

    const comment = await CommentRepository.create({
      postId,
      authorId: user.id,
      content: content.trim(),
      parentId: parsedParentId,
      replyToId: parsedReplyToId
    })

    // 帖子评论数 +1
    await PostRepository.incrementCommentCount(postId, 1)

    // 通知接收方：回复时通知线程中的目标用户，否则通知帖子作者；接收方为自己时跳过
    let notifyUserId: number | null = null
    if (parsedReplyToId && parsedReplyToId !== user.id) {
      notifyUserId = parsedReplyToId
    } else if (parentComment) {
      if (parentComment.authorId !== user.id) {
        notifyUserId = parentComment.authorId
      }
    } else if (post.authorId !== user.id) {
      notifyUserId = post.authorId
    }

    if (notifyUserId !== null) {
      const notificationTitle = parsedParentId || parsedReplyToId
        ? user.nickname + ' 回复了你'
        : user.nickname + ' 评论了你的帖子'
      await NotificationRepository.create({
        userId: notifyUserId,
        type: NotificationType.COMMENT,
        actorId: user.id,
        targetType: 'post',
        targetId: postId,
        title: notificationTitle,
        content
      })
    }

    return sendSuccess(res, comment, '评论成功', 201)
  } catch (error) {
    const message = error instanceof Error ? error.message : '创建评论失败'
    return sendInternalError(res, message)
  }
}
