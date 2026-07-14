import type { NextApiRequest, NextApiResponse } from 'next'
import { FollowRepository } from '@/db/repositories'
import type { Follow } from '@prisma/client'
import {
  sendSuccess,
  sendPaginated,
  sendError,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse, PaginatedData } from '@/lib/api-response'

/**
 * 用户关注列表接口
 *
 * GET /api/users/:id/following?page=1&pageSize=20
 * 无需登录鉴权
 */

/** 关注记录（含被关注用户的精简信息，由仓库 include 注入） */
type FollowWithFollowing = Follow & {
  following: {
    id: number
    nickname: string
    avatar: string | null
    bio: string | null
  }
}

/** 关注列表返回的单条数据 */
type FollowingItem = {
  id: number
  nickname: string
  avatar: string | null
  bio: string | null
  followedAt: string
}

/**
 * 从请求中解析目标用户 ID
 * @param req Next.js 请求对象
 * @returns 解析后的数字 ID 或 null
 */
function parseUserId(req: NextApiRequest): number | null {
  const raw = Array.isArray(req.query.id) ? req.query.id[0] : req.query.id
  if (!raw) return null
  const id = Number(raw)
  return Number.isNaN(id) ? null : id
}

/**
 * 从请求中解析分页参数，并对每页数量做上限保护
 * @param req Next.js 请求对象
 * @returns 归一化后的页码与每页数量
 */
function parsePagination(req: NextApiRequest): { page: number; pageSize: number } {
  const rawPage = Array.isArray(req.query.page) ? req.query.page[0] : req.query.page
  const rawPageSize = Array.isArray(req.query.pageSize)
    ? req.query.pageSize[0]
    : req.query.pageSize
  const page = Math.max(1, Number(rawPage) || 1)
  const pageSize = Math.max(1, Math.min(100, Number(rawPageSize) || 20))
  return { page, pageSize }
}

/**
 * 处理用户关注列表查询请求
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<PaginatedData<FollowingItem>>>
) {
  if (req.method !== 'GET') {
    return sendError(res, '仅支持 GET 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const userId = parseUserId(req)
    if (!userId) {
      return sendError(res, '无效的用户 ID', 'INVALID_ID')
    }

    const { page, pageSize } = parsePagination(req)

    const { list, total } = await FollowRepository.findFollowing(userId, page, pageSize)

    const items: FollowingItem[] = (list as unknown as FollowWithFollowing[]).map((item) => ({
      id: item.following.id,
      nickname: item.following.nickname,
      avatar: item.following.avatar,
      bio: item.following.bio,
      followedAt: item.createdAt.toISOString()
    }))

    return sendPaginated(res, items, total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '获取关注列表失败'
    return sendInternalError(res, message)
  }
}
