import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser, isAdmin } from '@/lib/auth-context'
import { CircleRepository } from '@/db/repositories'
import type { UpdateCircleInput } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendForbidden,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import { validatePersistedImageUrl } from '@/lib/media-url'

/**
 * 圈子详情与更新接口
 *
 * GET /api/circles/:id   获取圈子详情（含拥有者与成员）
 * PUT /api/circles/:id   更新圈子信息（需登录，仅拥有者或管理员可操作）
 * Headers: Authorization: Bearer <token>  （仅 PUT 需要）
 */

/** 圈子详情数据类型（由 findDetailById 返回，含 owner 与 members 关联） */
type CircleDetail = NonNullable<
  Awaited<ReturnType<typeof CircleRepository.findDetailById>>
>

/** 圈子详情响应：附带当前用户成员态，并对非成员隐藏成员名单。 */
type CircleDetailResponse = Omit<CircleDetail, 'members'> & {
  members: CircleDetail['members']
  isMember: boolean
  membershipRole: CircleDetail['members'][number]['role'] | null
  canViewAllPosts: boolean
}

/** 圈子更新请求体 */
type UpdateCircleBody = {
  name?: unknown
  description?: unknown
  cover?: unknown
  themeColor?: unknown
  isPublic?: unknown
}

/**
 * 从请求中解析目标圈子 ID
 * @param req Next.js 请求对象
 * @returns 解析后的数字 ID 或 null
 */
function parseCircleId(req: NextApiRequest): number | null {
  const raw = Array.isArray(req.query.id) ? req.query.id[0] : req.query.id
  if (!raw) return null
  const id = Number(raw)
  return Number.isNaN(id) ? null : id
}

/**
 * 校验并构造圈子更新输入，仅保留实际传入且类型合法的字段
 * @param body 请求体
 * @param userId 当前操作用户 ID
 * @param existingCover 数据库中已有封面地址
 * @returns 校验结果与更新输入
 */
function buildUpdateInput(
  body: UpdateCircleBody,
  userId: number,
  existingCover: string | null
): { ok: true; input: UpdateCircleInput } | { ok: false; message: string; code: string } {
  const input: UpdateCircleInput = {}

  if (typeof body.name === 'string' && body.name.trim()) {
    input.name = body.name.trim()
  }
  if (typeof body.description === 'string') {
    input.description = body.description
  }
  if (typeof body.cover === 'string') {
    const coverResult = validatePersistedImageUrl(body.cover, userId, existingCover)
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
 * 根据当前登录用户构造圈子详情权限视图。
 * 非成员仅获得圈子基础信息，成员名单只对成员与平台管理员开放。
 */
async function buildDetailResponse(
  req: NextApiRequest,
  detail: CircleDetail
): Promise<CircleDetailResponse> {
  const currentUser = await getCurrentUser(req)
  const membership = currentUser
    ? await CircleRepository.findMembership(detail.id, currentUser.id)
    : null
  const canViewAllPosts = !!membership || isAdmin(currentUser)

  return {
    ...detail,
    members: canViewAllPosts ? detail.members : [],
    isMember: !!membership,
    membershipRole: membership ? membership.role : null,
    canViewAllPosts
  }
}

/**
 * 处理圈子详情查询与更新请求
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<CircleDetailResponse | { deleted: boolean }>>
) {
  if (req.method !== 'GET' && req.method !== 'PUT' && req.method !== 'DELETE') {
    return sendError(res, '仅支持 GET、PUT 或 DELETE 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const id = parseCircleId(req)
    if (!id) {
      return sendError(res, '无效的圈子 ID', 'INVALID_ID')
    }

    if (req.method === 'GET') {
      const detail = await CircleRepository.findDetailById(id)
      if (!detail) {
        return sendNotFound(res, '圈子不存在')
      }
      return sendSuccess(res, await buildDetailResponse(req, detail))
    }

    // PUT：更新圈子
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const circle = await CircleRepository.findById(id)
    if (!circle) {
      return sendNotFound(res, '圈子不存在')
    }

    // 仅拥有者或管理员可更新
    if (circle.ownerId !== user.id && !isAdmin(user)) {
      return sendForbidden(res, '只有圈子拥有者或管理员才能更新圈子信息')
    }

    if (req.method === 'PUT') {
      const built = buildUpdateInput(req.body as UpdateCircleBody, user.id, circle.cover)
      if (!built.ok) {
        return sendError(res, built.message, built.code)
      }
      await CircleRepository.update(id, built.input)

      // 重新查询详情，确保返回包含 owner 与 members 的完整数据
      const detail = await CircleRepository.findDetailById(id)
      if (!detail) {
        return sendNotFound(res, '圈子不存在')
      }

      return sendSuccess(res, await buildDetailResponse(req, detail), '圈子更新成功')
    }

    // DELETE：删除圈子
    if (req.method === 'DELETE') {
      const deleteUser = await getCurrentUser(req)
      if (!deleteUser) {
        return sendUnauthorized(res)
      }

      const circleToDelete = await CircleRepository.findById(id)
      if (!circleToDelete) {
        return sendNotFound(res, '圈子不存在')
      }

      // 仅拥有者或管理员可删除
      if (circleToDelete.ownerId !== deleteUser.id && !isAdmin(deleteUser)) {
        return sendForbidden(res, '只有圈子拥有者或管理员才能删除圈子')
      }

      await CircleRepository.delete(id)
      return sendSuccess(res, { deleted: true }, '圈子已删除')
    }

    return sendError(res, '不支持的请求方法', 'METHOD_NOT_ALLOWED', 405)
  } catch (error) {
    const message = error instanceof Error ? error.message : '圈子操作失败'
    return sendInternalError(res, message)
  }
}
