import { Prisma, CheckIn } from '@prisma/client'
import prisma from '@/lib/prisma'
import { toShanghaiDateKey } from '@/lib/business-date'

/**
 * 打卡查询选项
 */
export type CheckInListOptions = {
  page?: number
  pageSize?: number
  userId?: number
  circleId?: number
}

/** 打卡记录创建参数。 */
export type CreateCheckInInput = {
  userId: number
  postId: number
  checkInDate: string
  circleId?: number
  content?: string
  images?: string
}

/** 打卡写入所需的最小 Prisma 客户端。 */
type CheckInWriteDatabase = Pick<Prisma.TransactionClient, 'checkIn'>

/**
 * 打卡记录仓库
 *
 * 职责：
 * - 封装打卡记录的创建与查询
 * - 提供当日是否已打卡的判断
 */
export const CheckInRepository = {
  /**
   * 创建打卡记录
   * @param input 打卡记录参数
   * @param database 普通 Prisma 客户端或事务客户端
   * @returns 创建的打卡记录
   */
  async create(
    input: CreateCheckInInput,
    database: CheckInWriteDatabase = prisma
  ): Promise<CheckIn> {
    return database.checkIn.create({
      data: {
        userId: input.userId,
        postId: input.postId,
        checkInDate: input.checkInDate,
        circleId: input.circleId,
        content: input.content,
        images: input.images
      }
    })
  },

  /**
   * 根据北京时间业务日期查询用户打卡记录。
   * @param userId 用户 ID
   * @param checkInDate YYYY-MM-DD 业务日期
   * @returns 打卡记录或 null
   */
  async findByDateKey(userId: number, checkInDate: string): Promise<CheckIn | null> {
    return prisma.checkIn.findUnique({
      where: {
        userId_checkInDate: {
          userId,
          checkInDate
        }
      }
    })
  },

  /**
   * 查询用户今日是否已打卡（基于服务器当前时间）
   * @param userId 用户 ID
   * @returns 打卡记录或 null
   */
  async findTodayCheckIn(userId: number): Promise<CheckIn | null> {
    return this.findByDateKey(userId, toShanghaiDateKey())
  },

  /**
   * 分页查询打卡记录
   * @param options 查询选项
   * @returns 打卡列表与总数
   */
  async findMany(
    options: CheckInListOptions = {}
  ): Promise<{ list: CheckIn[]; total: number }> {
    const { page = 1, pageSize = 20, userId, circleId } = options
    const skip = (page - 1) * pageSize

    const where: Prisma.CheckInWhereInput = {}
    if (userId) where.userId = userId
    if (circleId) where.circleId = circleId

    const [list, total] = await Promise.all([
      prisma.checkIn.findMany({
        where,
        skip,
        take: pageSize,
        include: {
          user: {
            select: { id: true, nickname: true, avatar: true }
          },
          post: {
            select: {
              id: true,
              topic: true
            }
          }
        },
        orderBy: { createdAt: 'desc' }
      }),
      prisma.checkIn.count({ where })
    ])

    return { list, total }
  }
}
