import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser, toPublicUser } from '@/lib/auth-context'
import { UserRepository } from '@/db/repositories'
import { comparePassword, isValidChinesePhone } from '@/lib/password'
import { sendError, sendInternalError, sendSuccess, sendUnauthorized } from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

export default async function handler(req: NextApiRequest, res: NextApiResponse<ApiResponse>) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  const currentUser = await getCurrentUser(req)
  if (!currentUser) {
    return sendUnauthorized(res)
  }

  const { newPhone, password } = req.body || {}
  if (!newPhone || !isValidChinesePhone(newPhone)) {
    return sendError(res, '请输入正确的手机号', 'INVALID_PHONE')
  }
  if (newPhone === currentUser.phone) {
    return sendError(res, '新手机号不能与当前手机号相同', 'SAME_PHONE')
  }
  if (!password) {
    return sendError(res, '请输入登录密码验证身份', 'MISSING_PASSWORD')
  }

  try {
    const user = await UserRepository.findById(currentUser.id)
    if (!user || !user.password) {
      return sendError(res, '请先设置登录密码后再更换手机号', 'PASSWORD_NOT_SET', 409)
    }

    const matched = await comparePassword(password, user.password)
    if (!matched) {
      return sendError(res, '登录密码不正确', 'INVALID_PASSWORD', 403)
    }

    const existingUser = await UserRepository.findByPhone(newPhone)
    if (existingUser) {
      return sendError(res, '该手机号已被其他账号使用', 'PHONE_ALREADY_REGISTERED', 409)
    }

    const updatedUser = await UserRepository.updatePhone(user.id, newPhone)
    return sendSuccess(res, { user: toPublicUser(updatedUser) }, '手机号更换成功')
  } catch (error) {
    return sendInternalError(res, error instanceof Error ? error.message : '手机号更换失败')
  }
}
