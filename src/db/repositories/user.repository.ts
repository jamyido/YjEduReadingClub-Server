import { Prisma, User, Gender, UserStatus } from '@prisma/client'
import prisma from '@/lib/prisma'

/**
 * 用户创建输入参数
 */
export type CreateUserInput = {
  phone: string
  password?: string
  nickname: string
  avatar?: string
  bio?: string
  weappOpenId?: string
  unionId?: string
}

/**
 * 用户更新输入参数
 */
export type UpdateUserInput = Partial<{
  nickname: string
  avatar: string
  bio: string
  gender: string
  birthday: Date
  status: string
}>

/**
 * 用户仓库
 *
 * 职责：
 * - 封装用户相关的数据库 CRUD 操作
 * - 向上层服务提供类型安全的用户数据访问接口
 */
export const UserRepository = {
  /**
   * 根据 ID 查询用户
   * @param id 用户 ID
   * @returns 用户记录或 null
   */
  async findById(id: number): Promise<User | null> {
    return prisma.user.findUnique({
      where: { id }
    })
  },

  /**
   * 根据手机号查询用户
   * @param phone 手机号
   * @returns 用户记录或 null
   */
  async findByPhone(phone: string): Promise<User | null> {
    return prisma.user.findUnique({
      where: { phone }
    })
  },

  /**
   * 根据微信小程序 openid 查询用户
   * @param openId 微信 openid
   * @returns 用户记录或 null
   */
  async findByWeappOpenId(openId: string): Promise<User | null> {
    return prisma.user.findUnique({
      where: { weappOpenId: openId }
    })
  },

  /**
   * 根据 unionId 查询用户
   * @param unionId 微信 unionid
   * @returns 用户记录或 null
   */
  async findByUnionId(unionId: string): Promise<User | null> {
    return prisma.user.findUnique({
      where: { unionId }
    })
  },

  /**
   * 创建新用户
   * @param input 用户创建参数
   * @returns 创建后的用户记录
   */
  async create(input: CreateUserInput): Promise<User> {
    const data: Prisma.UserCreateInput = {
      phone: input.phone,
      nickname: input.nickname,
      password: input.password,
      avatar: input.avatar,
      bio: input.bio,
      weappOpenId: input.weappOpenId,
      unionId: input.unionId
    }

    return prisma.user.create({ data })
  },

  /**
   * 更新用户信息
   * @param id 用户 ID
   * @param input 用户更新参数
   * @returns 更新后的用户记录
   */
  async update(id: number, input: UpdateUserInput): Promise<User> {
    const data: Prisma.UserUpdateInput = {
      nickname: input.nickname,
      avatar: input.avatar,
      bio: input.bio,
      gender: input.gender as Gender,
      birthday: input.birthday,
      status: input.status as UserStatus
    }

    // 移除 undefined 字段，避免 Prisma 报错
    Object.keys(data).forEach((key) => {
      if ((data as Record<string, unknown>)[key] === undefined) {
        delete (data as Record<string, unknown>)[key]
      }
    })

    return prisma.user.update({
      where: { id },
      data
    })
  },

  /**
   * 更新用户密码
   * @param id 用户 ID
   * @param hashedPassword 哈希后的密码
   * @returns 更新后的用户记录
   */
  async updatePassword(id: number, hashedPassword: string): Promise<User> {
    return prisma.user.update({
      where: { id },
      data: { password: hashedPassword }
    })
  },

  async updatePhone(id: number, phone: string): Promise<User> {
    return prisma.user.update({
      where: { id },
      data: { phone }
    })
  },

  /**
   * 为已有用户绑定微信小程序 openid 与 unionId
   * 用于微信登录时发现手机号已注册的场景
   * @param id 用户 ID
   * @param weappOpenId 微信 openid
   * @param unionId 微信 unionid（可选）
   * @returns 更新后的用户记录
   */
  async bindWeappOpenId(
    id: number,
    weappOpenId: string,
    unionId?: string
  ): Promise<User> {
    const data: Prisma.UserUpdateInput = { weappOpenId }
    if (unionId) {
      data.unionId = unionId
    }
    return prisma.user.update({ where: { id }, data })
  },

  /**
   * 更新用户打卡信息（连续天数与最后打卡时间）
   * @param id 用户 ID
   * @param streakDays 连续打卡天数
   * @param lastCheckInAt 最后打卡时间
   * @returns 更新后的用户记录
   */
  async updateCheckInInfo(
    id: number,
    streakDays: number,
    lastCheckInAt: Date
  ): Promise<User> {
    return prisma.user.update({
      where: { id },
      data: { streakDays, lastCheckInAt }
    })
  },

  /**
   * 更新用户关注/粉丝计数
   * @param id 用户 ID
   * @param followingDelta 关注数变化量
   * @param followerDelta 粉丝数变化量
   * @returns 更新后的用户记录
   */
  async updateFollowCounts(
    id: number,
    followingDelta: number,
    followerDelta: number
  ): Promise<User> {
    return prisma.user.update({
      where: { id },
      data: {
        followingCount: { increment: followingDelta },
        followerCount: { increment: followerDelta }
      }
    })
  },

  /**
   * 分页查询用户列表
   * @param page 页码
   * @param pageSize 每页数量
   * @returns 用户列表与总数
   */
  async findMany(page: number = 1, pageSize: number = 20): Promise<{ list: User[]; total: number }> {
    const skip = (page - 1) * pageSize

    const [list, total] = await Promise.all([
      prisma.user.findMany({
        skip,
        take: pageSize,
        orderBy: { createdAt: 'desc' }
      }),
      prisma.user.count()
    ])

    return { list, total }
  },

  /**
   * 从指定 ID 列表中筛选状态正常的用户。
   * @param ids 候选用户 ID
   * @returns 可接收系统通知的用户 ID
   */
  async findActiveIds(ids: number[]): Promise<number[]> {
    if (ids.length === 0) {
      return []
    }
    const users = await prisma.user.findMany({
      where: {
        id: { in: ids },
        status: UserStatus.ACTIVE
      },
      select: { id: true },
      orderBy: { id: 'asc' }
    })
    return users.map(function (user) { return user.id })
  },

  /**
   * 查询全部状态正常的用户 ID，用于平台系统通知广播。
   * @returns 可接收系统通知的用户 ID
   */
  async findAllActiveIds(): Promise<number[]> {
    const users = await prisma.user.findMany({
      where: { status: UserStatus.ACTIVE },
      select: { id: true },
      orderBy: { id: 'asc' }
    })
    return users.map(function (user) { return user.id })
  },

}
