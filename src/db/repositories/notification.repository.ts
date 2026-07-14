import { Prisma, Notification, NotificationType } from '@prisma/client'
import prisma from '@/lib/prisma'

/**
 * 通知创建输入参数
 */
export type CreateNotificationInput = {
  userId: number
  type: NotificationType
  actorId?: number
  targetType?: string
  targetId?: number
  title: string
  content?: string
}

/** 批量通知创建参数，由接收者列表与共享通知内容组成。 */
export type CreateManyNotificationsInput = Omit<CreateNotificationInput, 'userId'>

/** 按通知类型统计的未读数量。 */
export type NotificationUnreadTypeCount = {
  type: NotificationType
  targetType: string | null
  count: number
}

/** 通知列表支持的产品分类。 */
export type NotificationListCategory = 'LIKE' | 'REPLY' | 'SYSTEM' | 'TASK'

/** 通知列表查询参数。 */
export type NotificationListOptions = {
  page?: number
  pageSize?: number
  onlyUnread?: boolean
  category?: NotificationListCategory
}

/**
 * 通知仓库
 *
 * 职责：
 * - 封装通知的创建与查询
 * - 提供未读数统计与批量标记已读
 */
export const NotificationRepository = {
  /**
   * 创建通知
   * @param input 通知创建参数
   * @returns 创建的通知记录
   */
  async create(input: CreateNotificationInput): Promise<Notification> {
    return prisma.notification.create({
      data: {
        userId: input.userId,
        type: input.type,
        actorId: input.actorId,
        targetType: input.targetType,
        targetId: input.targetId,
        title: input.title,
        content: input.content
      }
    })
  },

  /**
   * 为多个用户批量创建相同的系统或任务通知。
   * @param userIds 接收用户 ID
   * @param input 共享通知内容
   * @returns 实际创建的通知数量
   */
  async createMany(
    userIds: number[],
    input: CreateManyNotificationsInput
  ): Promise<number> {
    if (userIds.length === 0) {
      return 0
    }

    const result = await prisma.notification.createMany({
      data: userIds.map(function (userId) {
        return {
          userId,
          type: input.type,
          actorId: input.actorId,
          targetType: input.targetType,
          targetId: input.targetId,
          title: input.title,
          content: input.content
        }
      })
    })
    return result.count
  },

  /**
   * 查询用户通知列表（分页）
   * @param userId 用户 ID
   * @param page 页码
   * @param pageSize 每页数量
   * @param onlyUnread 是否仅查未读
   * @returns 通知列表与总数
   */
  async findMany(
    userId: number,
    options: NotificationListOptions = {}
  ): Promise<{ list: Notification[]; total: number }> {
    const {
      page = 1,
      pageSize = 20,
      onlyUnread = false,
      category
    } = options
    const skip = (page - 1) * pageSize
    const where: Prisma.NotificationWhereInput = { userId }
    if (onlyUnread) {
      where.isRead = false
    }

    if (category === 'LIKE') {
      where.type = NotificationType.LIKE
    } else if (category === 'REPLY') {
      where.type = NotificationType.COMMENT
    } else if (category === 'TASK') {
      where.OR = [
        { type: NotificationType.TASK },
        { type: NotificationType.SYSTEM, targetType: 'task' }
      ]
    } else if (category === 'SYSTEM') {
      where.OR = [
        { type: NotificationType.FOLLOW },
        { type: NotificationType.CIRCLE_INVITE },
        { type: NotificationType.SYSTEM, targetType: null },
        { type: NotificationType.SYSTEM, targetType: { not: 'task' } }
      ]
    }

    const [list, total] = await Promise.all([
      prisma.notification.findMany({
        where,
        skip,
        take: pageSize,
        orderBy: [
          { createdAt: 'desc' },
          { id: 'desc' }
        ]
      }),
      prisma.notification.count({ where })
    ])

    return { list, total }
  },

  /**
   * 查询用户未读通知数
   * @param userId 用户 ID
   * @returns 未读通知数
   */
  async countUnread(userId: number): Promise<number> {
    return prisma.notification.count({
      where: { userId, isRead: false }
    })
  },

  /**
   * 统计用户全部通知数量。
   * @param userId 用户 ID
   * @returns 通知总数
   */
  async countAll(userId: number): Promise<number> {
    return prisma.notification.count({ where: { userId } })
  },

  /**
   * 按通知类型统计用户的未读数量。
   * @param userId 用户 ID
   * @returns 各通知类型的未读数量
   */
  async countUnreadByType(userId: number): Promise<NotificationUnreadTypeCount[]> {
    const groups = await prisma.notification.groupBy({
      by: ['type', 'targetType'],
      where: { userId, isRead: false },
      _count: { _all: true }
    })
    return groups.map(function (group) {
      return {
        type: group.type,
        targetType: group.targetType,
        count: group._count._all
      }
    })
  },

  /**
   * 将单条通知标记为已读
   * @param id 通知 ID
   * @param userId 用户 ID（用于权限校验）
   * @returns 更新后的通知记录
   */
  async markRead(id: number, userId: number): Promise<Notification | null> {
    const notification = await prisma.notification.findFirst({
      where: { id, userId }
    })
    if (!notification) return null

    return prisma.notification.update({
      where: { id },
      data: { isRead: true }
    })
  },

  /**
   * 将用户所有通知标记为已读
   * @param userId 用户 ID
   * @returns 更新的通知数量
   */
  async markAllRead(userId: number): Promise<number> {
    const result = await prisma.notification.updateMany({
      where: { userId, isRead: false },
      data: { isRead: true }
    })
    return result.count
  }
}
