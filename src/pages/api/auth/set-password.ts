import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser, toPublicUser } from '@/lib/auth-context'
import { UserRepository } from '@/db/repositories'
import { hashPassword, validatePasswordStrength } from '@/lib/password'
import { signToken } from '@/lib/jwt'
import {
  sendSuccess,
  sendError,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 设置密码响应数据
 */
type SetPasswordData = {
  user: ReturnType<typeof toPublicUser>
  hasPassword: boolean
}

/**
 * 设置密码接口（微信登录用户补充密码）
 *
 * 适用场景：
 * - 微信小程序一键登录的用户初始无密码
 * - 用户在个人中心主动设置密码后，可用手机号+密码在 H5 端登录
 *
 * POST /api/auth/set-password
 * Headers: Authorization: Bearer <token>
 * Body: { password: string }
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<SetPasswordData>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  const user = await getCurrentUser(req)
  if (!user) {
    return sendUnauthorized(res)
  }

  const { password } = req.body

  if (!password) {
    return sendError(res, '请输入密码', 'MISSING_PASSWORD')
  }

  const strengthError = validatePasswordStrength(password)
  if (strengthError) {
    return sendError(res, strengthError, 'WEAK_PASSWORD')
  }

  try {
    const storedUser = await UserRepository.findById(user.id)
    if (!storedUser) {
      return sendUnauthorized(res)
    }
    if (storedUser.password) {
      return sendError(res, '该账号已设置密码，请使用修改密码功能', 'PASSWORD_ALREADY_SET', 409)
    }

    const hashedPassword = await hashPassword(password)
    const updatedUser = await UserRepository.updatePassword(user.id, hashedPassword)

    const newToken = signToken({
      userId: updatedUser.id,
      phone: updatedUser.phone,
      role: updatedUser.role
    })

    res.setHeader('X-New-Token', newToken)

    return sendSuccess(
      res,
      {
        user: toPublicUser(updatedUser),
        hasPassword: true
      },
      '密码设置成功'
    )
  } catch (error) {
    const message = error instanceof Error ? error.message : '设置密码失败'
    return sendInternalError(res, message)
  }
}
