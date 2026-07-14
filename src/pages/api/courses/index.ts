import type { NextApiRequest, NextApiResponse } from 'next'
import { CourseRepository } from '@/db/repositories'
import {
  sendError,
  sendPaginated,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 课程列表接口
 *
 * GET /api/courses  分页查询课程列表
 * 查询参数：page、pageSize、circleId、creatorId
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
    const page = Number(req.query.page) || 1
    const pageSize = Number(req.query.pageSize) || 20
    const circleId = req.query.circleId ? Number(req.query.circleId) : undefined
    const creatorId = req.query.creatorId
      ? Number(req.query.creatorId)
      : undefined

    const { list, total } = await CourseRepository.findMany({
      page,
      pageSize,
      circleId,
      creatorId
    })

    return sendPaginated(res, list, total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询课程列表失败'
    return sendInternalError(res, message)
  }
}
