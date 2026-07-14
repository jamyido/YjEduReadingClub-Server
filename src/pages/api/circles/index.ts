import type { NextApiRequest, NextApiResponse } from 'next'
import type { Circle } from '@prisma/client'
import { getCurrentUser, isAdmin } from '@/lib/auth-context'
import { CircleRepository } from '@/db/repositories'
import type {
  CreateCircleInput,
  CircleListOptions
} from '@/db/repositories'
import {
  sendSuccess,
  sendPaginated,
  sendError,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse, PaginatedData } from '@/lib/api-response'
import { validatePersistedImageUrl } from '@/lib/media-url'

/**
 * 圈子列表与创建接口
 *
 * GET  /api/circles?page=1&pageSize=20&keyword=&ownerId=   分页查询圈子列表
 * POST /api/circles                                        创建圈子（需登录）
 * Headers: Authorization: Bearer <token>  （仅 POST 需要）
 */

/** 圈子创建请求体 */
type CreateCircleBody = {
  name: unknown
  description?: unknown
  cover?: unknown
  themeColor?: unknown
  isPublic?: unknown
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
 * 从请求中解析查询参数并构造圈子列表查询选项
 * @param req Next.js 请求对象
 * @returns 圈子列表查询选项
 */
function parseListOptions(req: NextApiRequest): CircleListOptions {
  const { page, pageSize } = parsePagination(req)

  const rawKeyword = Array.isArray(req.query.keyword)
    ? req.query.keyword[0]
    : req.query.keyword
  const keyword = rawKeyword && String(rawKeyword).trim() ? String(rawKeyword).trim() : undefined

  const rawOwnerId = Array.isArray(req.query.ownerId)
    ? req.query.ownerId[0]
    : req.query.ownerId
  const ownerIdNum = rawOwnerId ? Number(rawOwnerId) : NaN
  const ownerId = Number.isNaN(ownerIdNum) ? undefined : ownerIdNum

  return { page, pageSize, keyword, ownerId }
}

/**
 * 校验并构造圈子创建输入
 * @param body 请求体
 * @param ownerId 创建者用户 ID
 * @returns 校验结果，成功返回输入对象，失败返回错误信息
 */
function buildCreateInput(
  body: CreateCircleBody,
  ownerId: number
): { ok: true; input: CreateCircleInput } | { ok: false; message: string; code: string } {
  const name = typeof body.name === 'string' ? body.name.trim() : ''
  if (!name) {
    return { ok: false, message: '圈子名称不能为空', code: 'MISSING_NAME' }
  }
  if (name.length > 100) {
    return { ok: false, message: '圈子名称不能超过 100 个字符', code: 'NAME_TOO_LONG' }
  }

  const input: CreateCircleInput = {
    name,
    ownerId: 0
  }

  if (typeof body.description === 'string') {
    input.description = body.description
  }
  if (typeof body.cover === 'string') {
    const coverResult = validatePersistedImageUrl(body.cover, ownerId)
    if (!coverResult.ok) {
      return coverResult
    }
    input.cover = coverResult.url
  }
  if (typeof body.themeColor === 'string') {
    input.themeColor = body.themeColor
  }
  if (typeof body.isPublic === 'boolean') {
    input.isPublic = body.isPublic
  }

  return { ok: true, input }
}

/**
 * 处理圈子列表查询与创建请求
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<PaginatedData<Circle> | Circle>>
) {
  if (req.method !== 'GET' && req.method !== 'POST') {
    return sendError(res, '仅支持 GET 或 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    if (req.method === 'GET') {
      const options = parseListOptions(req)
      const { list, total } = await CircleRepository.findMany(options)
      return sendPaginated(res, list, total, options.page || 1, options.pageSize || 20)
    }

    // POST：创建圈子（仅平台管理员可操作）
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }
    if (!isAdmin(user)) {
      return sendError(res, '仅平台管理员可创建圈子', 'FORBIDDEN', 403)
    }

    const built = buildCreateInput(req.body as CreateCircleBody, user.id)
    if (!built.ok) {
      return sendError(res, built.message, built.code)
    }

    const created = await CircleRepository.create({
      ...built.input,
      ownerId: user.id
    })

    return sendSuccess(res, created, '圈子创建成功', 201)
  } catch (error) {
    const message = error instanceof Error ? error.message : '圈子操作失败'
    return sendInternalError(res, message)
  }
}
