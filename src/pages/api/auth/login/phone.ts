import type { NextApiRequest, NextApiResponse } from 'next'
import { UserRepository } from '@/db/repositories'
import { comparePassword, isValidChinesePhone } from '@/lib/password'
import { signToken } from '@/lib/jwt'
import { toPublicUser } from '@/lib/auth-context'
import {
  sendSuccess,
  sendError,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 手机号密码登录响应数据
 */
type PhoneLoginData = {
  token: string
  user: ReturnType<typeof toPublicUser>
  hasPassword: boolean
}

/**
 * 手机号 + 密码登录接口（H5 端使用）
 *
 * POST /api/auth/login/phone
 * Body: { phone: string, password: string }
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<PhoneLoginData>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  const { phone, password } = req.body

  if (!phone || !password) {
    return sendError(res, '请输入手机号和密码', 'MISSING_CREDENTIALS')
  }

  if (!isValidChinesePhone(phone)) {
    return sendError(res, '手机号格式不正确', 'INVALID_PHONE')
  }

  try {
    const user = await UserRepository.findByPhone(phone)

    if (!user) {
      return sendError(res, '手机号或密码错误', 'INVALID_CREDENTIALS', 401)
    }

    if (user.status === 'BANNED') {
      return sendError(res, '账号已被封禁', 'ACCOUNT_BANNED', 403)
    }

    if (!user.password) {
      return sendError(
        res,
        '该账号尚未设置密码，请使用微信登录后设置密码',
        'PASSWORD_NOT_SET',
        403
      )
    }

    const isMatch = await comparePassword(password, user.password)
    if (!isMatch) {
      return sendError(res, '手机号或密码错误', 'INVALID_CREDENTIALS', 401)
    }

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
      '登录成功'
    )
  } catch (error) {
    const message = error instanceof Error ? error.message : '登录失败'
    return sendInternalError(res, message)
  }
}
