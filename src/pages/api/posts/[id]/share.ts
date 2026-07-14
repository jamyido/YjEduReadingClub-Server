import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { PostRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 从查询参数中解析帖子 ID。
 * @param value 原始查询参数（可能为数组）
 * @returns 帖子 ID 或 null
 */
function parsePostId(value: string | string[] | undefined): number | null {
  const raw = Array.isArray(value) ? value[0] : value
  if (!raw) return null
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

/**
 * 记录帖子转发意图。
 * 微信不会回传最终是否成功发送给好友，因此接口只统计用户主动打开转发面板的行为。
 *
 * POST /api/posts/:id/share
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  const postId = parsePostId(req.query.id)
  if (postId === null) {
    return sendError(res, '帖子 ID 格式不正确', 'INVALID_ID')
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const post = await PostRepository.findById(postId)
    if (!post || post.status === 1) {
      return sendNotFound(res, '帖子不存在或已删除')
    }

    const updated = await PostRepository.incrementShareCount(postId, 1)
    return sendSuccess(res, { shareCount: updated.shareCount })
  } catch (error) {
    const message = error instanceof Error ? error.message : '记录转发失败'
    return sendInternalError(res, message)
  }
}
