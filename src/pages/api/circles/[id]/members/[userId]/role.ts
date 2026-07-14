import type { NextApiRequest, NextApiResponse } from 'next'
import { CircleMemberRole } from '@prisma/client'
import { getCurrentUser, isAdmin } from '@/lib/auth-context'
import { CircleRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendForbidden,
  sendNotFound,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 圈子成员角色管理接口
 *
 * PUT /api/circles/:id/members/:userId/role
 * Body: { role: 'MEMBER' | 'MODERATOR' | 'OWNER' }
 * Headers: Authorization: Bearer <token>
 *
 * 权限：仅圈子拥有者或平台管理员可操作；
 * 将成员任命为圈主时，会同时转让圈子拥有权。
 */

/** 角色更新请求体 */
type UpdateRoleBody = {
  role?: unknown
}

/** 合法的角色取值集合 */
const validRoles: CircleMemberRole[] = ['MEMBER', 'MODERATOR', 'OWNER']

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
 * 从请求中解析目标用户 ID
 * @param req Next.js 请求对象
 * @returns 解析后的数字 ID 或 null
 */
function parseUserId(req: NextApiRequest): number | null {
  const raw = Array.isArray(req.query.userId) ? req.query.userId[0] : req.query.userId
  if (!raw) return null
  const id = Number(raw)
  return Number.isNaN(id) ? null : id
}

/**
 * 校验并解析请求体中的目标角色
 * @param body 请求体
 * @returns 合法角色或 null
 */
function parseRole(body: UpdateRoleBody): CircleMemberRole | null {
  if (typeof body.role !== 'string') {
    return null
  }
  const role = body.role.toUpperCase()
  if (validRoles.indexOf(role as CircleMemberRole) < 0) {
    return null
  }
  return role as CircleMemberRole
}

/**
 * 处理圈子成员角色更新请求
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<{ role: CircleMemberRole }>>
) {
  if (req.method !== 'PUT') {
    return sendError(res, '仅支持 PUT 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const circleId = parseCircleId(req)
    if (!circleId) {
      return sendError(res, '无效的圈子 ID', 'INVALID_ID')
    }

    const targetUserId = parseUserId(req)
    if (!targetUserId) {
      return sendError(res, '无效的用户 ID', 'INVALID_USER_ID')
    }

    const circle = await CircleRepository.findById(circleId)
    if (!circle) {
      return sendNotFound(res, '圈子不存在')
    }

    // 仅拥有者或平台管理员可变更角色
    if (circle.ownerId !== user.id && !isAdmin(user)) {
      return sendForbidden(res, '只有圈子拥有者或管理员才能变更成员角色')
    }

    // 不能修改自己的角色，避免权限悬空
    if (targetUserId === user.id) {
      return sendError(res, '不能变更自己的角色', 'CANNOT_UPDATE_SELF', 400)
    }

    const targetMembership = await CircleRepository.findMembership(circleId, targetUserId)
    if (!targetMembership) {
      return sendError(res, '该用户不是圈子成员', 'NOT_MEMBER', 409)
    }

    const role = parseRole(req.body as UpdateRoleBody)
    if (!role) {
      return sendError(res, '无效的角色取值', 'INVALID_ROLE', 400)
    }

    // 任命为圈主时执行拥有权转让
    if (role === 'OWNER') {
      await CircleRepository.transferOwnership(circleId, circle.ownerId, targetUserId)
    } else {
      await CircleRepository.updateMemberRole(circleId, targetUserId, role)
    }

    return sendSuccess(res, { role }, '成员角色更新成功')
  } catch (error) {
    const message = error instanceof Error ? error.message : '成员角色更新失败'
    return sendInternalError(res, message)
  }
}
