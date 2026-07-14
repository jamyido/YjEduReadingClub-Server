import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { CourseRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import type { CourseProgress } from '@prisma/client'

/** 更新进度请求体 */
interface UpdateProgressBody {
  currentChapterId?: number
  completedChapterIds?: string
  progress?: number
  isCompleted?: boolean
}

/** 进度更新响应数据 */
interface ProgressResult {
  progress: CourseProgress
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
 * 课程学习进度更新接口
 *
 * POST /api/courses/[id]/progress  更新当前用户在某课程的学习进度（需登录）
 * Body: { currentChapterId?, completedChapterIds?, progress?, isCompleted? }
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<ProgressResult>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const courseId = parseCourseIdParam(req.query.id)
    if (!courseId) {
      return sendError(res, '课程 ID 不合法', 'INVALID_COURSE_ID', 400)
    }

    // 校验课程存在
    const course = await CourseRepository.findById(courseId)
    if (!course) {
      return sendNotFound(res, '课程不存在')
    }

    const body = (req.body || {}) as UpdateProgressBody

    const updatedProgress = await CourseRepository.upsertProgress(
      user.id,
      courseId,
      {
        currentChapterId: body.currentChapterId,
        completedChapterIds: body.completedChapterIds,
        progress: body.progress,
        isCompleted: body.isCompleted
      }
    )

    return sendSuccess(res, { progress: updatedProgress })
  } catch (error) {
    const message = error instanceof Error ? error.message : '更新学习进度失败'
    return sendInternalError(res, message)
  }
}
