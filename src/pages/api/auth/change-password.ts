import type { NextApiRequest, NextApiResponse } from 'next'
import { getCurrentUser, toPublicUser } from '@/lib/auth-context'
import { UserRepository } from '@/db/repositories'
import { comparePassword, hashPassword, validatePasswordStrength } from '@/lib/password'
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

  const { currentPassword, newPassword } = req.body || {}
  if (!currentPassword || !newPassword) {
    return sendError(res, '请输入原密码和新密码', 'MISSING_PASSWORD')
  }

  const strengthError = validatePasswordStrength(newPassword)
  if (strengthError) {
    return sendError(res, strengthError, 'WEAK_PASSWORD')
  }
  if (currentPassword === newPassword) {
    return sendError(res, '新密码不能与原密码相同', 'SAME_PASSWORD')
  }

  try {
    const user = await UserRepository.findById(currentUser.id)
    if (!user || !user.password) {
      return sendError(res, '该账号尚未设置密码', 'PASSWORD_NOT_SET', 409)
    }

    const matched = await comparePassword(currentPassword, user.password)
    if (!matched) {
      return sendError(res, '原密码不正确', 'INVALID_CURRENT_PASSWORD', 403)
    }

    const hashedPassword = await hashPassword(newPassword)
    const updatedUser = await UserRepository.updatePassword(user.id, hashedPassword)
    return sendSuccess(res, { user: toPublicUser(updatedUser), hasPassword: true }, '密码修改成功')
  } catch (error) {
    return sendInternalError(res, error instanceof Error ? error.message : '密码修改失败')
  }
}
