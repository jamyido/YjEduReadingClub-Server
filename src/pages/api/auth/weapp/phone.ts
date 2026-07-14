import type { NextApiRequest, NextApiResponse } from 'next'
import type { User } from '@prisma/client'
import { getPhoneNumber } from '@/lib/wechat'
import { verifyWeappTempToken, signToken } from '@/lib/jwt'
import { UserRepository } from '@/db/repositories'
import { toPublicUser } from '@/lib/auth-context'
import { generateNickname } from '@/lib/nickname'
import { sendSuccess, sendError, sendInternalError } from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 微信手机号授权响应数据
 */
type WeappPhoneData = {
  token: string
  user: ReturnType<typeof toPublicUser>
  isNewUser: boolean
  hasPassword: boolean
}

/**
 * 微信小程序手机号授权接口（一键登录第二步）
 *
 * 流程：
 * 1. 前端使用 button[open-type=getPhoneNumber] 引导用户授权手机号
 * 2. 将回调中的 code 与第一步获得的 tempToken 一起发送到本接口
 * 3. 后端验证 tempToken，提取 openid 与 session_key
 * 4. 通过微信 API 用 code 换取手机号
 * 5. 检查手机号是否已注册：
 *    - 已注册 → 将 openid 绑定到已有用户，返回 JWT
 *    - 未注册 → 创建新用户（手机号 + openid，无密码），返回 JWT
 *
 * POST /api/auth/weapp/phone
 * Body: { code: string, tempToken: string }
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<WeappPhoneData>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  const { code, tempToken } = req.body

  if (!code) {
    return sendError(res, '缺少手机号授权 code', 'MISSING_CODE')
  }

  if (!tempToken) {
    return sendError(res, '缺少临时登录凭证 tempToken', 'MISSING_TEMP_TOKEN')
  }

  const tempPayload = verifyWeappTempToken(tempToken)
  if (!tempPayload) {
    return sendError(res, '临时登录凭证已过期，请重新登录', 'TEMP_TOKEN_EXPIRED', 401)
  }

  try {
    const phone = await getPhoneNumber(code)

    const existingUser = await UserRepository.findByPhone(phone)

    let user: User
    let isNewUser = false

    if (existingUser) {
      if (existingUser.status === 'BANNED') {
        return sendError(res, '账号已被封禁', 'ACCOUNT_BANNED', 403)
      }

      if (!existingUser.weappOpenId) {
        await UserRepository.bindWeappOpenId(
          existingUser.id,
          tempPayload.openid,
          tempPayload.unionid
        )
      }

      user = existingUser
    } else {
      isNewUser = true
      user = await UserRepository.create({
        phone,
        nickname: generateNickname(),
        weappOpenId: tempPayload.openid,
        unionId: tempPayload.unionid
      })
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
        isNewUser,
        hasPassword: !!user.password
      },
      isNewUser ? '注册成功' : '登录成功'
    )
  } catch (error) {
    const message = error instanceof Error ? error.message : '获取手机号失败'
    return sendInternalError(res, message)
  }
}
