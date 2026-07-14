import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { PostRepository, NotificationRepository } from '@/db/repositories'
import {
  sendSuccess,
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
 * 帖子点赞与取消点赞接口
 *
 * POST   /api/posts/:id/like   点赞帖子（需登录），返回 { liked: true, likeCount }
 * DELETE /api/posts/:id/like   取消点赞（需登录），返回 { liked: false, likeCount }
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

  if (req.method === 'POST') {
    return handleLike(req, res, id)
  }
  if (req.method === 'DELETE') {
    return handleUnlike(req, res, id)
  }
  return sendError(res, '仅支持 POST/DELETE 请求', 'METHOD_NOT_ALLOWED', 405)
}

/**
 * 点赞帖子
 *
 * 流程：
 * 1. 校验登录态与帖子存在性
 * 2. 查重：已点赞则返回 409
 * 3. 创建点赞记录并增加帖子点赞数
 * 4. 通知帖子作者（自己点赞自己不通知）
 *
 * @param req 请求对象
 * @param res 响应对象
 * @param postId 帖子 ID
 */
async function handleLike(
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

    const existing = await PostRepository.findLike(user.id, 'post', postId)
    if (existing) {
      return sendError(res, '已经点赞过该帖子', 'ALREADY_LIKED', 409)
    }

    await PostRepository.createLike(user.id, 'post', postId)
    const updated = await PostRepository.incrementLikeCount(postId, 1)

    // 通知帖子作者收到点赞（点赞者本人即作者时跳过）
    if (post.authorId !== user.id) {
      await NotificationRepository.create({
        userId: post.authorId,
        type: NotificationType.LIKE,
        actorId: user.id,
        targetType: 'post',
        targetId: postId,
        title: user.nickname + ' 赞了你的帖子',
        content: post.title || post.content.slice(0, 80)
      })
    }

    return sendSuccess(res, { liked: true, likeCount: updated.likeCount })
  } catch (error) {
    const message = error instanceof Error ? error.message : '点赞失败'
    return sendInternalError(res, message)
  }
}

/**
 * 取消点赞帖子
 *
 * 流程：
 * 1. 校验登录态与帖子存在性
 * 2. 查重：未点赞则返回 409
 * 3. 删除点赞记录并减少帖子点赞数
 *
 * @param req 请求对象
 * @param res 响应对象
 * @param postId 帖子 ID
 */
async function handleUnlike(
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

    const existing = await PostRepository.findLike(user.id, 'post', postId)
    if (!existing) {
      return sendError(res, '尚未点赞该帖子', 'NOT_LIKED', 409)
    }

    await PostRepository.deleteLike(user.id, 'post', postId)
    const updated = await PostRepository.incrementLikeCount(postId, -1)

    return sendSuccess(res, { liked: false, likeCount: updated.likeCount })
  } catch (error) {
    const message = error instanceof Error ? error.message : '取消点赞失败'
    return sendInternalError(res, message)
  }
}
