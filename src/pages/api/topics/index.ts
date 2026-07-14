import type { NextApiRequest, NextApiResponse } from 'next'
import { TopicRepository } from '@/db/repositories'
import {
  sendError,
  sendInternalError,
  sendPaginated
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 将查询参数解析为受上限保护的正整数。
 * @param value 原始查询参数
 * @param fallback 默认值
 * @param maximum 最大值
 * @returns 合法正整数
 */
function parsePositiveInt(
  value: string | string[] | undefined,
  fallback: number,
  maximum: number
): number {
  const raw = Array.isArray(value) ? value[0] : value
  if (!raw) return fallback
  const parsed = Number.parseInt(raw, 10)
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback
  return Math.min(parsed, maximum)
}

/**
 * 获取启用话题列表。
 * GET /api/topics?page=1&pageSize=20&query=关键词
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
    const page = parsePositiveInt(req.query.page, 1, 100000)
    const pageSize = parsePositiveInt(req.query.pageSize, 20, 100)
    const rawQuery = Array.isArray(req.query.query)
      ? req.query.query[0]
      : req.query.query
    const result = await TopicRepository.findMany({
      page,
      pageSize,
      query: rawQuery || ''
    })

    const list = result.list.map(function (topic) {
      return {
        id: topic.id,
        slug: topic.slug,
        title: topic.title,
        description: topic.description,
        status: topic.status,
        sort: topic.sort,
        postCount: topic._count.posts,
        createdAt: topic.createdAt,
        updatedAt: topic.updatedAt
      }
    })

    return sendPaginated(res, list, result.total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询话题列表失败'
    return sendInternalError(res, message)
  }
}
