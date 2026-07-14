import type { NextApiRequest, NextApiResponse } from 'next'
import { UserRepository } from '@/db/repositories'
import {
  hashPassword,
  validatePasswordStrength,
  isValidChinesePhone
} from '@/lib/password'
import { signToken, verifyWeappTempToken } from '@/lib/jwt'
import { toPublicUser } from '@/lib/auth-context'
import {
  sendSuccess,
  sendError,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'
import { validatePersistedImageUrl } from '@/lib/media-url'

/**
 * 注册响应数据
 */
type RegisterData = {
  token: string
  user: ReturnType<typeof toPublicUser>
  hasPassword: boolean
}

/**
 * 手机号 + 密码注册接口（H5 端与微信小程序端共用）
 *
 * POST /api/auth/register
 * Body: {
 *   phone: string,
 *   password: string,
 *   nickname?: string,
 *   avatar?: string,        // 头像 URL（上传接口返回的路径）
 *   tempToken?: string      // 微信小程序临时 token，用于绑定 openid
 * }
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<RegisterData>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  const { phone, password, nickname, avatar, tempToken } = req.body
  let persistedAvatar: string | undefined

  if (!phone || !password) {
    return sendError(res, '请输入手机号和密码', 'MISSING_CREDENTIALS')
  }

  if (!isValidChinesePhone(phone)) {
    return sendError(res, '手机号格式不正确', 'INVALID_PHONE')
  }

  const strengthError = validatePasswordStrength(password)
  if (strengthError) {
    return sendError(res, strengthError, 'WEAK_PASSWORD')
  }

  if (avatar !== undefined && avatar !== null) {
    if (typeof avatar !== 'string') {
      return sendError(res, '头像地址格式无效', 'INVALID_MEDIA_URL')
    }
    const avatarResult = validatePersistedImageUrl(avatar)
    if (!avatarResult.ok) {
      return sendError(res, avatarResult.message, avatarResult.code)
    }
    persistedAvatar = avatarResult.url || undefined
  }

  // 解析微信临时 token（可选），用于绑定 openid
  let weappOpenId: string | undefined
  let unionId: string | undefined
  if (tempToken) {
    const tempPayload = verifyWeappTempToken(tempToken)
    if (!tempPayload) {
      return sendError(res, '微信登录凭证已过期，请重新登录', 'TEMP_TOKEN_EXPIRED', 401)
    }
    weappOpenId = tempPayload.openid
    unionId = tempPayload.unionid
  }

  try {
    const existingUser = await UserRepository.findByPhone(phone)
    if (existingUser) {
      return sendError(res, '该手机号已注册', 'PHONE_ALREADY_REGISTERED', 409)
    }

    // 若提供了 openid，检查是否已被其他用户绑定
    if (weappOpenId) {
      const existingWeappUser = await UserRepository.findByWeappOpenId(weappOpenId)
      if (existingWeappUser) {
        return sendError(res, '该微信号已绑定其他账号', 'WEAPP_ALREADY_BOUND', 409)
      }
    }

    const hashedPassword = await hashPassword(password)
    const user = await UserRepository.create({
      phone,
      password: hashedPassword,
      nickname: nickname && nickname.trim() ? nickname.trim() : `书友${phone.slice(-4)}`,
      avatar: persistedAvatar,
      weappOpenId,
      unionId
    })

    const token = signToken({
      userId: user.id,
      phone: user.phone,
      role: user.role
    })

    return sendSuccess(
      res,
      {
        token,
        user: toPublicUser(user),
        hasPassword: true
      },
      '注册成功'
    )
  } catch (error) {
    const message = error instanceof Error ? error.message : '注册失败'
    return sendInternalError(res, message)
  }
}
