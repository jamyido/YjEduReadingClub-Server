import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser } from '@/lib/auth-context'
import { CircleRepository } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 当前用户已加入的圈子列表接口
 *
 * GET /api/circles/mine
 * Headers: Authorization: Bearer <token>
 */

/** 用户圈子成员关系记录（含圈子信息，由 findUserCircles 返回） */
type UserCircleMembership = Awaited<
  ReturnType<typeof CircleRepository.findUserCircles>
>[number]

/** 我的圈子列表返回的单条数据 */
type MyCircleItem = {
  id: number
  userId: number
  circleId: number
  role: UserCircleMembership['role']
  createdAt: string
  updatedAt: string
  joinedAt: string
  circle: {
    id: number
    name: string
    description: string | null
    cover: string | null
    themeColor: string | null
    isPublic: boolean
    memberCount: number
    postCount: number
    ownerId: number
    createdAt: string
    updatedAt: string
  }
}

/**
 * 将成员关系记录映射为对外返回的圈子列表项
 * @param membership 圈子成员关系记录
 * @returns 精简后的列表项数据
 */
function toMyCircleItem(membership: UserCircleMembership): MyCircleItem {
  return {
    id: membership.id,
    userId: membership.userId,
    circleId: membership.circleId,
    role: membership.role,
    createdAt: membership.createdAt.toISOString(),
    updatedAt: membership.updatedAt.toISOString(),
    joinedAt: membership.createdAt.toISOString(),
    circle: {
      id: membership.circle.id,
      name: membership.circle.name,
      description: membership.circle.description,
      cover: membership.circle.cover,
      themeColor: membership.circle.themeColor,
      isPublic: membership.circle.isPublic,
      memberCount: membership.circle.memberCount,
      postCount: membership.circle.postCount,
      ownerId: membership.circle.ownerId,
      createdAt: membership.circle.createdAt.toISOString(),
      updatedAt: membership.circle.updatedAt.toISOString()
    }
  }
}

/**
 * 处理当前用户已加入圈子列表查询请求
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<MyCircleItem[]>>
) {
  if (req.method !== 'GET') {
    return sendError(res, '仅支持 GET 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const memberships = await CircleRepository.findUserCircles(user.id)
    const items = memberships.map(toMyCircleItem)

    return sendSuccess(res, items)
  } catch (error) {
    const message = error instanceof Error ? error.message : '获取我的圈子失败'
    return sendInternalError(res, message)
  }
}
