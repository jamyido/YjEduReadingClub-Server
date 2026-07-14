import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { CheckInRepository } from '@/db/repositories'
import { PostService, PostServiceError } from '@/services/post.service'
import {
  sendSuccess,
  sendError,
  sendPaginated,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import type { CheckIn } from '@prisma/client'
import { validatePersistedImageUrl } from '@/lib/media-url'

/** 打卡请求体 */
interface CheckInBody {
  circleId?: unknown
  content?: unknown
  images?: unknown
}

/** 打卡响应数据 */
interface CheckInResult {
  checkIn: CheckIn
  streak: number
}

/**
 * 将旧打卡接口的 images 字段归一化为 URL 数组。
 * 同时兼容单个 URL、URL 数组和历史 JSON 数组字符串。
 * @param images 原始 images 字段
 * @returns 图片 URL 数组
 */
function normalizeImageUrls(images: unknown): string[] {
  if (Array.isArray(images)) {
    return images.filter(function (item): item is string {
      return typeof item === 'string' && item.trim().length > 0
    }).map(function (item) { return item.trim() })
  }
  if (typeof images !== 'string' || images.trim().length === 0) {
    return []
  }

  const value = images.trim()
  if (value.charAt(0) === '[') {
    try {
      return normalizeImageUrls(JSON.parse(value))
    } catch (error) {
      return []
    }
  }
  return [value]
}

/**
 * 打卡相关接口入口
 *
 * GET  /api/checkin   分页查询打卡记录
 * POST /api/checkin   每日打卡（需登录）
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  switch (req.method) {
    case 'GET':
      return handleList(req, res)
    case 'POST':
      return handleCreate(req, res)
    default:
      return sendError(res, '仅支持 GET / POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }
}

/**
 * 分页查询打卡记录
 * GET /api/checkin?page=1&pageSize=20&userId=&circleId=
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
async function handleList(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  try {
    const page = Number(req.query.page) || 1
    const pageSize = Number(req.query.pageSize) || 20
    const userId = req.query.userId ? Number(req.query.userId) : undefined
    const circleId = req.query.circleId ? Number(req.query.circleId) : undefined

    const { list, total } = await CheckInRepository.findMany({
      page,
      pageSize,
      userId,
      circleId
    })

    return sendPaginated(res, list, total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询打卡列表失败'
    return sendInternalError(res, message)
  }
}

/**
 * 兼容旧客户端的每日打卡入口。
 * 实际委托 PostService 创建一条默认“打卡挑战”帖子，并复用同一事务计算连续天数。
 * POST /api/checkin  Body: { circleId?, content?, images? }
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
async function handleCreate(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<CheckInResult>>
) {
  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const body = (req.body || {}) as CheckInBody
    let circleId: number | undefined
    if (body.circleId != null && body.circleId !== '') {
      const parsedCircleId = Number(body.circleId)
      if (!Number.isInteger(parsedCircleId) || parsedCircleId <= 0) {
        return sendError(res, '圈子 ID 格式不正确', 'INVALID_CIRCLE_ID')
      }
      circleId = parsedCircleId
    }

    const content = typeof body.content === 'string' && body.content.trim()
      ? body.content.trim()
      : '完成今日打卡'
    const imageUrls = normalizeImageUrls(body.images)
    if (imageUrls.length > 9) {
      return sendError(res, '打卡图片最多 9 张', 'INVALID_MEDIAS')
    }

    const persistedImageUrls: string[] = []
    for (let index = 0; index < imageUrls.length; index += 1) {
      const imageResult = validatePersistedImageUrl(imageUrls[index], user.id)
      if (!imageResult.ok) {
        return sendError(res, imageResult.message, imageResult.code)
      }
      persistedImageUrls.push(imageResult.url)
    }

    const result = await PostService.create({
      authorId: user.id,
      circleId,
      type: persistedImageUrls.length > 0 ? 'IMAGE' : 'TEXT',
      content,
      medias: persistedImageUrls.map(function (url, index) {
        return { type: 'image', url, sort: index }
      }),
      requireNewCheckIn: true
    })

    if (!result.checkIn) {
      return sendInternalError(res, '打卡记录创建失败')
    }

    return sendSuccess(
      res,
      {
        checkIn: result.checkIn,
        streak: result.streakDays
      },
      '打卡成功',
      201
    )
  } catch (error) {
    if (error instanceof PostServiceError) {
      return sendError(res, error.message, error.code, error.statusCode)
    }
    const message = error instanceof Error ? error.message : '打卡失败'
    return sendInternalError(res, message)
  }
}
