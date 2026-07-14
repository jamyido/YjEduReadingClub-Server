import type { NextApiRequest, NextApiResponse } from 'next'

import { CircleRepository, PostRepository } from '@/db/repositories'
import {
  sendError,
  sendForbidden,
  sendInternalError,
  sendNotFound,
  sendPaginated,
  sendUnauthorized
} from '@/lib/api-response'
import type { ApiResponse, PaginatedData } from '@/lib/api-response'
import { getCurrentUser, isAdmin } from '@/lib/auth-context'

/** 单条圈子累计发帖天数排名响应。 */
type CirclePostDaysRankingItem = {
  rank: number
  userId: number
  nickname: string
  avatar: string | null
  cumulativePostDays: number
  postCount: number
  lastPostAt: string
}

/**
 * 从请求中解析正整数查询参数。
 * @param value 原始查询参数
 * @param fallback 解析失败时的默认值
 * @param maximum 允许的最大值
 * @returns 归一化后的正整数
 */
function parsePositiveInteger(
  value: string | string[] | undefined,
  fallback: number,
  maximum: number
): number {
  const raw = Array.isArray(value) ? value[0] : value
  const parsed = raw ? Number.parseInt(raw, 10) : NaN
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback
  }
  return Math.min(Math.floor(parsed), maximum)
}

/**
 * 从动态路由中解析圈子 ID。
 * @param req Next.js 请求对象
 * @returns 有效圈子 ID，解析失败时返回 0
 */
function parseCircleId(req: NextApiRequest): number {
  const raw = Array.isArray(req.query.id) ? req.query.id[0] : req.query.id
  const parsed = raw ? Number(raw) : 0
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 0
}

/**
 * 查询当前圈子内用户累计发帖天数排行榜。
 * 仅圈子成员或平台管理员可访问，避免泄露私域圈子的用户活跃信息。
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<PaginatedData<CirclePostDaysRankingItem>>>
) {
  if (req.method !== 'GET') {
    return sendError(res, '仅支持 GET 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const currentUser = await getCurrentUser(req)
    if (!currentUser) {
      return sendUnauthorized(res)
    }

    const circleId = parseCircleId(req)
    if (!circleId) {
      return sendError(res, '无效的圈子 ID', 'INVALID_CIRCLE_ID')
    }

    const circle = await CircleRepository.findById(circleId)
    if (!circle) {
      return sendNotFound(res, '圈子不存在')
    }

    const administrator = isAdmin(currentUser)
    const membership = administrator
      ? null
      : await CircleRepository.findMembership(circleId, currentUser.id)
    if (!administrator && !membership) {
      return sendForbidden(res, '加入圈子后才能查看累计发帖天数排行榜')
    }

    const page = parsePositiveInteger(req.query.page, 1, 100000)
    const pageSize = parsePositiveInteger(req.query.pageSize, 50, 100)
    const result = await PostRepository.findCirclePostDaysRanking(circleId, {
      page,
      pageSize
    })
    const list = result.list.map(function (item, index) {
      return {
        rank: (page - 1) * pageSize + index + 1,
        userId: item.userId,
        nickname: item.nickname,
        avatar: item.avatar,
        cumulativePostDays: item.cumulativePostDays,
        postCount: item.postCount,
        lastPostAt: item.lastPostAt.toISOString()
      }
    })

    return sendPaginated(res, list, result.total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '获取累计发帖天数排行榜失败'
    return sendInternalError(res, message)
  }
}
