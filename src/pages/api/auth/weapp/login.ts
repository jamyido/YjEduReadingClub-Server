import type { NextApiRequest, NextApiResponse } from 'next'
import { code2Session } from '@/lib/wechat'
import { signToken, signWeappTempToken } from '@/lib/jwt'
import { UserRepository } from '@/db/repositories'
import { toPublicUser } from '@/lib/auth-context'
import { sendSuccess, sendError, sendInternalError } from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 微信登录响应数据
 */
type WeappLoginData =
  | {
      needPhone: false
      token: string
      user: ReturnType<typeof toPublicUser>
      isNewUser: boolean
      hasPassword: boolean
    }
  | {
      needPhone: true
      tempToken: string
      message: string
    }

/**
 * 微信小程序登录接口（一键登录第一步）
 *
 * 流程：
 * 1. 小程序端调用 wx.login() 获取 code
 * 2. 将 code 发送到本接口
 * 3. 后端通过 code2session 换取 openid 与 session_key
 * 4. 若 openid 已存在用户记录 → 直接登录，返回 JWT
 * 5. 若 openid 不存在用户记录 → 返回临时 token，前端引导用户授权手机号
 *
 * POST /api/auth/weapp/login
 * Body: { code: string }
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<WeappLoginData>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  const { code } = req.body

  if (!code) {
    return sendError(res, '缺少微信登录 code', 'MISSING_CODE')
  }

  try {
    const session = await code2Session(code)

    const existingUser = await UserRepository.findByWeappOpenId(session.openid)

    if (existingUser) {
      if (existingUser.status === 'BANNED') {
        return sendError(res, '账号已被封禁', 'ACCOUNT_BANNED', 403)
      }

      const token = signToken({
        userId: existingUser.id,
        phone: existingUser.phone,
        role: existingUser.role
      })

      return sendSuccess(res, {
        needPhone: false,
        token,
        user: toPublicUser(existingUser),
        isNewUser: false,
        hasPassword: !!existingUser.password
      })
    }

    const tempToken = signWeappTempToken({
      openid: session.openid,
      sessionKey: session.session_key,
      unionid: session.unionid
    })

    return sendSuccess(
      res,
      {
        needPhone: true,
        tempToken,
        message: '请授权手机号完成注册'
      },
      '新用户，请授权手机号'
    )
  } catch (error) {
    const message = error instanceof Error ? error.message : '微信登录失败'
    return sendInternalError(res, message)
  }
}
