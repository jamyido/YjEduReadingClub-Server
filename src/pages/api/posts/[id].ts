import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser, isAdmin } from '@/lib/auth-context'
import { CircleRepository, PostRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendForbidden,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

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
 * 帖子详情与删除接口
 *
 * GET    /api/posts/:id   获取帖子详情（含作者、圈子、媒体、评论）
 * DELETE /api/posts/:id   软删除帖子（仅作者或管理员可操作）
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
    return handleDetail(req, res, id)
  }
  if (req.method === 'DELETE') {
    return handleDelete(req, res, id)
  }
  return sendError(res, '仅支持 GET/DELETE 请求', 'METHOD_NOT_ALLOWED', 405)
}

/**
 * 获取帖子详情
 * @param req 请求对象
 * @param res 响应对象
 * @param id 帖子 ID
 */
async function handleDetail(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>,
  id: number
) {
  try {
    const post = await PostRepository.findDetailById(id)
    if (!post || post.status === 1) {
      return sendNotFound(res, '帖子不存在或已删除')
    }
    const currentUser = await getCurrentUser(req)
    if (post.circleId && !isAdmin(currentUser)) {
      const membership = currentUser
        ? await CircleRepository.findMembership(post.circleId, currentUser.id)
        : null
      if (!membership) {
        return sendForbidden(res, '加入圈子后才能查看完整帖子')
      }
    }
    const liked = currentUser
      ? await PostRepository.findLike(currentUser.id, 'post', id)
      : null
    return sendSuccess(res, {
      ...post,
      isLiked: !!liked
    })
  } catch (error) {
    const message = error instanceof Error ? error.message : '获取帖子详情失败'
    return sendInternalError(res, message)
  }
}

/**
 * 软删除帖子
 *
 * 权限校验：仅帖子作者或管理员可删除；已删除的帖子返回 404。
 *
 * @param req 请求对象
 * @param res 响应对象
 * @param id 帖子 ID
 */
async function handleDelete(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>,
  id: number
) {
  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const post = await PostRepository.findById(id)
    if (!post || post.status === 1) {
      return sendNotFound(res, '帖子不存在或已删除')
    }

    if (post.authorId !== user.id && !isAdmin(user)) {
      return sendForbidden(res, '无权删除他人帖子')
    }

    await PostRepository.softDelete(id)
    return sendSuccess(res, { id }, '删除成功')
  } catch (error) {
    const message = error instanceof Error ? error.message : '删除帖子失败'
    return sendInternalError(res, message)
  }
}
