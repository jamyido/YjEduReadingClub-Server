import type { NextApiRequest, NextApiResponse } from 'next'
import { NotificationType } from '@prisma/client'
import { getCurrentUser, isAdmin } from '@/lib/auth-context'
import {
  CircleRepository,
  NotificationRepository,
  UserRepository
} from '@/db/repositories'
import {
  sendError,
  sendForbidden,
  sendSuccess,
  sendUnauthorized,
  sendInternalError
} from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

type NotificationAudience = 'ALL' | 'USERS' | 'CIRCLE'

/** 管理员派发系统或任务通知的请求体。 */
interface DispatchNotificationBody {
  type?: unknown
  audience?: unknown
  userIds?: unknown
  circleId?: unknown
  title?: unknown
  content?: unknown
  targetType?: unknown
  targetId?: unknown
}

/** 校验后的通知派发参数。 */
interface DispatchNotificationInput {
  type: NotificationType
  audience: NotificationAudience
  userIds: number[]
  circleId?: number
  title: string
  content?: string
  targetType?: string
  targetId?: number
}

/** 派发成功响应。 */
interface DispatchNotificationResult {
  type: 'SYSTEM' | 'TASK'
  audience: NotificationAudience
  recipientCount: number
}

/** 通知派发参数校验失败结果。 */
interface DispatchValidationError {
  ok: false
  message: string
  code: string
  statusCode?: number
}

/**
 * 将 userIds 归一化为去重的正整数列表。
 * @param value 原始请求字段
 * @returns 用户 ID 列表
 */
function normalizeUserIds(value: unknown): number[] {
  if (!Array.isArray(value)) {
    return []
  }
  const userIds: number[] = []
  value.forEach(function (item) {
    if (
      typeof item === 'number'
      && Number.isInteger(item)
      && item > 0
      && userIds.indexOf(item) < 0
    ) {
      userIds.push(item)
    }
  })
  return userIds
}

/**
 * 校验并构造通知派发参数。
 * @param body 原始请求体
 * @returns 成功时返回派发参数，否则返回错误信息
 */
function buildDispatchInput(
  body: DispatchNotificationBody
): { ok: true; input: DispatchNotificationInput } | DispatchValidationError {
  const rawType = typeof body.type === 'string' ? body.type.toUpperCase() : ''
  if (rawType !== 'SYSTEM' && rawType !== 'TASK') {
    return { ok: false, message: '仅支持派发系统通知或任务通知', code: 'INVALID_NOTIFICATION_TYPE' }
  }

  const rawAudience = typeof body.audience === 'string' ? body.audience.toUpperCase() : ''
  if (rawAudience !== 'ALL' && rawAudience !== 'USERS' && rawAudience !== 'CIRCLE') {
    return { ok: false, message: '通知接收范围无效', code: 'INVALID_AUDIENCE' }
  }

  const title = typeof body.title === 'string' ? body.title.trim() : ''
  if (!title) {
    return { ok: false, message: '通知标题不能为空', code: 'MISSING_TITLE' }
  }
  if (title.length > 200) {
    return { ok: false, message: '通知标题不能超过 200 个字符', code: 'TITLE_TOO_LONG' }
  }

  const content = typeof body.content === 'string' ? body.content.trim() : undefined
  if (content && content.length > 5000) {
    return { ok: false, message: '通知内容不能超过 5000 个字符', code: 'CONTENT_TOO_LONG' }
  }
  if (rawType === 'TASK' && !content) {
    return { ok: false, message: '任务通知必须包含任务说明', code: 'MISSING_TASK_CONTENT' }
  }

  const userIds = normalizeUserIds(body.userIds)
  if (rawAudience === 'USERS') {
    if (!Array.isArray(body.userIds) || body.userIds.length === 0 || userIds.length === 0) {
      return { ok: false, message: '请至少指定一个接收用户', code: 'MISSING_RECIPIENTS' }
    }
    if (body.userIds.length > 500 || userIds.length > 500) {
      return { ok: false, message: '单次最多指定 500 个接收用户', code: 'TOO_MANY_RECIPIENTS' }
    }
    if (userIds.length !== body.userIds.length) {
      return { ok: false, message: '接收用户 ID 必须是互不重复的正整数', code: 'INVALID_RECIPIENTS' }
    }
  }

  const circleId = typeof body.circleId === 'number'
    && Number.isInteger(body.circleId)
    && body.circleId > 0
    ? body.circleId
    : undefined
  if (rawAudience === 'CIRCLE' && !circleId) {
    return { ok: false, message: '请指定有效的圈子 ID', code: 'INVALID_CIRCLE_ID' }
  }

  const rawTargetType = typeof body.targetType === 'string' ? body.targetType.trim() : ''
  if (rawTargetType.length > 20) {
    return { ok: false, message: '关联目标类型不能超过 20 个字符', code: 'TARGET_TYPE_TOO_LONG' }
  }
  if (rawType === 'SYSTEM' && rawTargetType.toLowerCase() === 'task') {
    return { ok: false, message: '任务提醒请使用 TASK 通知类型', code: 'TASK_TYPE_REQUIRED' }
  }

  const parsedTargetId = body.targetId === undefined || body.targetId === null
    ? undefined
    : body.targetId
  if (
    parsedTargetId !== undefined
    && (
      typeof parsedTargetId !== 'number'
      || !Number.isInteger(parsedTargetId)
      || parsedTargetId <= 0
    )
  ) {
    return { ok: false, message: '关联目标 ID 必须是正整数', code: 'INVALID_TARGET_ID' }
  }

  return {
    ok: true,
    input: {
      type: rawType === 'TASK' ? NotificationType.TASK : NotificationType.SYSTEM,
      audience: rawAudience,
      userIds,
      circleId,
      title,
      content: content || undefined,
      targetType: rawTargetType || (rawType === 'TASK' ? 'task' : undefined),
      targetId: parsedTargetId
    }
  }
}

/**
 * 根据通知接收范围解析状态正常的接收用户。
 * @param input 已校验的派发参数
 * @returns 接收用户 ID 或错误信息
 */
async function resolveRecipientIds(
  input: DispatchNotificationInput
): Promise<{ ok: true; userIds: number[] } | DispatchValidationError> {
  if (input.audience === 'ALL') {
    const allUserIds = await UserRepository.findAllActiveIds()
    if (allUserIds.length === 0) {
      return { ok: false, message: '当前没有可接收通知的用户', code: 'NO_RECIPIENTS', statusCode: 409 }
    }
    return { ok: true, userIds: allUserIds }
  }

  if (input.audience === 'USERS') {
    const activeUserIds = await UserRepository.findActiveIds(input.userIds)
    if (activeUserIds.length !== input.userIds.length) {
      return { ok: false, message: '部分接收用户不存在或已被禁用', code: 'INVALID_RECIPIENTS' }
    }
    return { ok: true, userIds: activeUserIds }
  }

  const candidateIds = await CircleRepository.findNotificationRecipientIds(input.circleId || 0)
  if (candidateIds === null) {
    return { ok: false, message: '圈子不存在', code: 'CIRCLE_NOT_FOUND', statusCode: 404 }
  }
  const activeCircleUserIds = await UserRepository.findActiveIds(candidateIds)
  if (activeCircleUserIds.length === 0) {
    return { ok: false, message: '圈子内没有可接收通知的用户', code: 'NO_RECIPIENTS', statusCode: 409 }
  }
  return { ok: true, userIds: activeCircleUserIds }
}

/**
 * 管理员通知派发接口。
 * POST /api/notifications/dispatch 仅允许平台管理员派发 SYSTEM 或 TASK 通知。
 * @param req Next.js 请求对象
 * @param res Next.js 响应对象
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<DispatchNotificationResult>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  try {
    const user = await getCurrentUser(req)
    if (!user) {
      return sendUnauthorized(res)
    }
    if (!isAdmin(user)) {
      return sendForbidden(res, '仅平台管理员可派发系统通知和任务通知')
    }

    const built = buildDispatchInput((req.body || {}) as DispatchNotificationBody)
    if (!built.ok) {
      return sendError(res, built.message, built.code, built.statusCode || 400)
    }

    const recipients = await resolveRecipientIds(built.input)
    if (!recipients.ok) {
      return sendError(
        res,
        recipients.message,
        recipients.code,
        recipients.statusCode || 400
      )
    }

    const recipientCount = await NotificationRepository.createMany(
      recipients.userIds,
      {
        type: built.input.type,
        targetType: built.input.targetType,
        targetId: built.input.targetId,
        title: built.input.title,
        content: built.input.content
      }
    )

    return sendSuccess(res, {
      type: built.input.type === NotificationType.TASK ? 'TASK' : 'SYSTEM',
      audience: built.input.audience,
      recipientCount
    }, '通知派发成功', 201)
  } catch (error) {
    const message = error instanceof Error ? error.message : '通知派发失败'
    return sendInternalError(res, message)
  }
}
