import { Prisma, Message } from '@prisma/client'
import prisma from '@/lib/prisma'

/**
 * 消息查询选项
 */
export type MessageListOptions = {
  page?: number
  pageSize?: number
  userId?: number
  otherUserId?: number
  onlyUnread?: boolean
}

/**
 * 私信消息仓库
 *
 * 职责：
 * - 封装私信消息的数据库操作
 * - 提供会话列表、消息列表、未读数查询
 */
export const MessageRepository = {
  /**
   * 发送私信
   * @param senderId 发送者 ID
   * @param receiverId 接收者 ID
   * @param content 消息内容
   * @returns 创建的消息记录
   */
  async send(
    senderId: number,
    receiverId: number,
    content: string
  ): Promise<Message> {
    return prisma.message.create({
      data: { senderId, receiverId, content }
    })
  },

  /**
   * 查询两个用户之间的私聊记录（分页）
   * @param userId 当前用户 ID
   * @param otherUserId 对方用户 ID
   * @param page 页码
   * @param pageSize 每页数量
   * @returns 消息列表与总数
   */
  async findConversation(
    userId: number,
    otherUserId: number,
    page: number = 1,
    pageSize: number = 50
  ): Promise<{ list: Message[]; total: number }> {
    const skip = (page - 1) * pageSize
    const where: Prisma.MessageWhereInput = {
      OR: [
        { senderId: userId, receiverId: otherUserId },
        { senderId: otherUserId, receiverId: userId }
      ]
    }

    const [list, total] = await Promise.all([
      prisma.message.findMany({
        where,
        skip,
        take: pageSize,
        orderBy: { createdAt: 'asc' }
      }),
      prisma.message.count({ where })
    ])

    return { list, total }
  },

  /**
   * 查询用户的消息列表（分页）
   * @param options 查询选项
   * @returns 消息列表与总数
   */
  async findMany(
    options: MessageListOptions = {}
  ): Promise<{ list: Message[]; total: number }> {
    const {
      page = 1,
      pageSize = 20,
      userId,
      otherUserId,
      onlyUnread
    } = options
    const skip = (page - 1) * pageSize

    const where: Prisma.MessageWhereInput = {}

    if (userId && otherUserId) {
      where.OR = [
        { senderId: userId, receiverId: otherUserId },
        { senderId: otherUserId, receiverId: userId }
      ]
    } else if (userId) {
      where.OR = [{ senderId: userId }, { receiverId: userId }]
    }

    if (onlyUnread) {
      where.isRead = false
    }

    const [list, total] = await Promise.all([
      prisma.message.findMany({
        where,
        skip,
        take: pageSize,
        orderBy: { createdAt: 'desc' }
      }),
      prisma.message.count({ where })
    ])

    return { list, total }
  },

  /**
   * 查询用户未读消息总数
   * @param userId 用户 ID
   * @returns 未读消息数
   */
  async countUnread(userId: number): Promise<number> {
    return prisma.message.count({
      where: { receiverId: userId, isRead: false }
    })
  },

  /**
   * 将指定用户发来的消息标记为已读
   * @param receiverId 接收者 ID（当前用户）
   * @param senderId 发送者 ID（对方用户）
   * @returns 更新的消息数量
   */
  async markConversationRead(
    receiverId: number,
    senderId: number
  ): Promise<number> {
    const result = await prisma.message.updateMany({
      where: { receiverId, senderId, isRead: false },
      data: { isRead: true }
    })
    return result.count
  },

  /**
   * 将用户所有未读消息标记为已读
   * @param userId 用户 ID
   * @returns 更新的消息数量
   */
  async markAllRead(userId: number): Promise<number> {
    const result = await prisma.message.updateMany({
      where: { receiverId: userId, isRead: false },
      data: { isRead: true }
    })
    return result.count
  }
}
