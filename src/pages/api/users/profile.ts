import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser, toPublicUser } from '@/lib/auth-context'
import type { PublicUser } from '@/lib/auth-context'
import { UserRepository } from '@/db/repositories'
import type { UpdateUserInput } from '@/db/repositories'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import { validatePersistedImageUrl } from '@/lib/media-url'

/**
 * 更新当前用户资料接口
 *
 * PUT /api/users/profile
 * Headers: Authorization: Bearer <token>
 * Body: { nickname?, avatar?, bio?, gender?, birthday? }
 */

/** 合法的性别取值，与 Prisma Gender 枚举保持一致 */
const VALID_GENDERS = ['UNKNOWN', 'MALE', 'FEMALE']

/**
 * 处理当前用户资料更新请求
 * 仅更新请求体中实际传入的字段，未传入的字段保持原值不变。
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<PublicUser>>
) {
  if (req.method !== 'PUT') {
    return sendError(res, '仅支持 PUT 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }

    const { nickname, avatar, bio, gender, birthday } = req.body

    // 构造更新输入，仅包含实际传入的字段
    const updateInput: UpdateUserInput = {}

    if (typeof nickname === 'string' && nickname.trim()) {
      updateInput.nickname = nickname.trim()
    }

    if (typeof avatar === 'string') {
      const avatarResult = validatePersistedImageUrl(avatar, user.id, user.avatar)
      if (!avatarResult.ok) {
        return sendError(res, avatarResult.message, avatarResult.code)
      }
      updateInput.avatar = avatarResult.url
    }

    if (typeof bio === 'string') {
      updateInput.bio = bio
    }

    if (gender !== undefined && gender !== null) {
      if (typeof gender !== 'string' || !VALID_GENDERS.includes(gender)) {
        return sendError(res, '性别取值无效', 'INVALID_GENDER')
      }
      updateInput.gender = gender
    }

    if (birthday !== undefined && birthday !== null) {
      const parsed = new Date(birthday)
      if (Number.isNaN(parsed.getTime())) {
        return sendError(res, '生日格式无效', 'INVALID_BIRTHDAY')
      }
      updateInput.birthday = parsed
    }

    const updatedUser = await UserRepository.update(user.id, updateInput)

    return sendSuccess(res, toPublicUser(updatedUser), '资料更新成功')
  } catch (error) {
    const message = error instanceof Error ? error.message : '更新资料失败'
    return sendInternalError(res, message)
  }
}
