import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { CourseRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/** 课程详情响应数据 */
interface CourseDetail {
  course: Awaited<ReturnType<typeof CourseRepository.findById>>
  progress: Awaited<ReturnType<typeof CourseRepository.findProgress>>
}

/**
 * 解析动态路由参数中的课程 ID
 * @param query 路由查询参数（string | string[] | undefined）
 * @returns 解析出的正整数 ID 或 null
 */
function parseCourseIdParam(
  query: string | string[] | undefined
): number | null {
  if (!query) return null
  const value = Array.isArray(query) ? query[0] : query
  const id = Number(value)
  return Number.isFinite(id) && id > 0 ? id : null
}

/**
 * 课程详情接口
 *
 * GET /api/courses/[id]  获取课程详情（含章节），登录用户附带学习进度
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<CourseDetail>>
) {
  if (req.method !== 'GET') {
    return sendError(res, '仅支持 GET 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const courseId = parseCourseIdParam(req.query.id)
    if (!courseId) {
      return sendError(res, '课程 ID 不合法', 'INVALID_COURSE_ID', 400)
    }

    const course = await CourseRepository.findById(courseId)
    if (!course) {
      return sendNotFound(res, '课程不存在')
    }

    // 登录用户附带学习进度（认证可选）
    const user = await getCurrentUser(req)
    const progress = user
      ? await CourseRepository.findProgress(user.id, courseId)
      : null

    return sendSuccess(res, { course, progress })
  } catch (error) {
    const message = error instanceof Error ? error.message : '获取课程详情失败'
    return sendInternalError(res, message)
  }
}
