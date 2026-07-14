import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser, isAdmin } from '@/lib/auth-context'
import {
  PostRepository,
  CircleRepository
} from '@/db/repositories'
import { PostService, PostServiceError } from '@/services/post.service'
import {
  sendSuccess,
  sendPaginated,
  sendError,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import { validatePersistedImageUrl } from '@/lib/media-url'

/**
 * 将查询字符串参数解析为正整数
 * @param value 原始查询字符串（可能为数组）
 * @param fallback 解析失败或为空时的默认值
 * @returns 解析后的正整数
 */
function parsePositiveInt(
  value: string | string[] | undefined,
  fallback: number
): number {
  const raw = Array.isArray(value) ? value[0] : value
  if (!raw) return fallback
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

/** 非成员预览正文的最大字符数。 */
const CIRCLE_PREVIEW_CONTENT_LENGTH = 160

/**
 * 截断非成员可见的帖子正文，避免通过列表接口取得完整圈内内容。
 * @param content 完整帖子正文
 * @returns 最多 160 个字符的预览正文
 */
function buildCirclePostPreview(content: string): string {
  return content.length > CIRCLE_PREVIEW_CONTENT_LENGTH
    ? content.slice(0, CIRCLE_PREVIEW_CONTENT_LENGTH) + '…'
    : content
}

/**
 * 帖子列表与创建接口
 *
 * GET  /api/posts   分页查询帖子列表（支持 circleId / authorId / topicId / type 过滤）
 * POST /api/posts   创建帖子（需登录）
 *
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  if (req.method === 'GET') {
    return handleList(req, res)
  }
  if (req.method === 'POST') {
    return handleCreate(req, res)
  }
  return sendError(res, '仅支持 GET/POST 请求', 'METHOD_NOT_ALLOWED', 405)
}

/**
 * 分页查询帖子列表
 * 圈子帖子对未登录或非成员只开放第一页最多 3 条预览。
 * @param req 请求对象，读取 page / pageSize / circleId / authorId / topicId / type 查询参数
 * @param res 响应对象
 */
async function handleList(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  try {
    let page = parsePositiveInt(req.query.page, 1)
    let pageSize = Math.min(parsePositiveInt(req.query.pageSize, 20), 100)
    const circleId = parsePositiveInt(req.query.circleId, 0) || undefined
    const authorId = parsePositiveInt(req.query.authorId, 0) || undefined
    const topicId = parsePositiveInt(req.query.topicId, 0) || undefined
    const currentUser = await getCurrentUser(req)
    const administrator = isAdmin(currentUser)
    const joinedCircleIds = new Set<number>()

    if (currentUser && !administrator) {
      const memberships = await CircleRepository.findUserCircles(currentUser.id)
      memberships.forEach(function (membership) {
        joinedCircleIds.add(membership.circleId)
      })
    }

    if (circleId && !administrator) {
      if (!joinedCircleIds.has(circleId)) {
        page = 1
        pageSize = Math.min(pageSize, 3)
      }
    }

    const rawType = Array.isArray(req.query.type) ? req.query.type[0] : req.query.type
    const type = rawType || undefined

    const { list, total } = await PostRepository.findMany({
      page,
      pageSize,
      circleId,
      authorId,
      topicId,
      type
    })

    const likedPostIds = currentUser
      ? await PostRepository.findLikedPostIds(currentUser.id, list.map(function (post) { return post.id }))
      : []
    const likedPostIdSet = new Set(likedPostIds)
    const enrichedList = list.map(function (post) {
      const enrichedPost = {
        ...post,
        isLiked: likedPostIdSet.has(post.id)
      }
      const shouldPreview = !!post.circleId
        && !administrator
        && !joinedCircleIds.has(post.circleId)
      if (!shouldPreview) {
        return {
          ...enrichedPost,
          isPreview: false
        }
      }
      return {
        ...enrichedPost,
        content: buildCirclePostPreview(post.content),
        linkUrl: null,
        medias: post.medias.slice(0, 1),
        isPreview: true
      }
    })

    return sendPaginated(res, enrichedList, total, page, pageSize)
  } catch (error) {
    const message = error instanceof Error ? error.message : '查询帖子列表失败'
    return sendInternalError(res, message)
  }
}

/**
 * 创建帖子
 *
 * 流程：
 * 1. 校验登录态、正文、媒体与圈子成员身份
 * 2. 解析有效话题；旧客户端未传 topicId 时回退到打卡挑战
 * 3. 在同一事务写入帖子并更新圈子计数
 * 4. 打卡挑战当天首次额外写入打卡与连续天数
 *
 * @param req 请求对象，Body: { circleId, topicId?, type?, title?, content, linkUrl?, medias?: [{type, url}] }
 * @param res 响应对象
 */
async function handleCreate(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse>
) {
  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const body = req.body || {}
    const { circleId, topicId, type, title, content, linkUrl, medias } = body

    if (!content || typeof content !== 'string' || content.trim().length === 0) {
      return sendError(res, '帖子内容不能为空', 'INVALID_CONTENT')
    }

    if (circleId == null || circleId === '') {
      return sendError(res, '发布帖子必须指定圈子', 'CIRCLE_ID_REQUIRED')
    }

    const parsedCircleId = Number(circleId)
    if (!Number.isInteger(parsedCircleId) || parsedCircleId <= 0) {
      return sendError(res, '圈子 ID 格式不正确', 'INVALID_CIRCLE_ID')
    }

    let parsedTopicId: number | undefined
    if (topicId != null && topicId !== '') {
      const num = Number(topicId)
      if (!Number.isInteger(num) || num <= 0) {
        return sendError(res, '话题 ID 格式不正确', 'INVALID_TOPIC_ID')
      }
      parsedTopicId = num
    }

    var safeMedias: Array<{ type: string; url: string; sort?: number }> = []
    if (medias !== undefined) {
      if (!Array.isArray(medias) || medias.length > 9) {
        return sendError(res, '帖子图片数量不正确', 'INVALID_MEDIAS')
      }

      for (var mediaIndex = 0; mediaIndex < medias.length; mediaIndex += 1) {
        var media = medias[mediaIndex]
        var mediaUrl = media && typeof media.url === 'string' ? media.url.trim() : ''
        if (!mediaUrl) {
          return sendError(res, '图片地址不能为空', 'INVALID_MEDIA_URL')
        }
        var mediaResult = validatePersistedImageUrl(mediaUrl, user.id)
        if (!mediaResult.ok) {
          return sendError(res, mediaResult.message, mediaResult.code)
        }

        safeMedias.push({
          type: media.type === 'video' ? 'video' : 'image',
          url: mediaResult.url,
          sort: mediaIndex
        })
      }
    }

    const result = await PostService.create({
      authorId: user.id,
      circleId: parsedCircleId,
      topicId: parsedTopicId,
      type,
      title,
      content: content.trim(),
      linkUrl,
      medias: safeMedias
    })

    return sendSuccess(res, result.post, '发布成功', 201)
  } catch (error) {
    if (error instanceof PostServiceError) {
      return sendError(res, error.message, error.code, error.statusCode)
    }
    const message = error instanceof Error ? error.message : '创建帖子失败'
    return sendInternalError(res, message)
  }
}
