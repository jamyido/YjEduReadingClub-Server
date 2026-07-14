import { Prisma, Circle, CircleMemberRole } from '@prisma/client'
import prisma from '@/lib/prisma'

/**
 * 圈子创建输入参数
 */
export type CreateCircleInput = {
  name: string
  description?: string
  cover?: string
  themeColor?: string
  isPublic?: boolean
  ownerId: number
}

/**
 * 圈子更新输入参数
 */
export type UpdateCircleInput = Partial<{
  name: string
  description: string
  cover: string
  themeColor: string
  isPublic: boolean
}>

/**
 * 圈子查询选项
 */
export type CircleListOptions = {
  page?: number
  pageSize?: number
  keyword?: string
  ownerId?: number
}

/**
 * 圈子仓库
 *
 * 职责：
 * - 封装圈子及其成员关系的数据库操作
 * - 提供创建、查询、更新、成员管理等接口
 */
export const CircleRepository = {
  /**
   * 根据 ID 查询圈子
   * @param id 圈子 ID
   * @returns 圈子记录或 null
   */
  async findById(id: number): Promise<Circle | null> {
    return prisma.circle.findUnique({
      where: { id }
    })
  },

  /**
   * 根据 ID 查询圈子详情（包含成员与拥有者）
   * @param id 圈子 ID
   * @returns 圈子详情或 null
   */
  async findDetailById(id: number) {
    return prisma.circle.findUnique({
      where: { id },
      include: {
        owner: {
          select: { id: true, nickname: true, avatar: true }
        },
        members: {
          include: {
            user: {
              select: { id: true, nickname: true, avatar: true }
            }
          }
        }
      }
    })
  },

  /**
   * 创建圈子
   * @param input 圈子创建参数
   * @returns 创建后的圈子记录
   */
  async create(input: CreateCircleInput): Promise<Circle> {
    const data: Prisma.CircleCreateInput = {
      name: input.name,
      description: input.description,
      cover: input.cover,
      themeColor: input.themeColor,
      isPublic: input.isPublic,
      owner: { connect: { id: input.ownerId } },
      members: {
        create: {
          userId: input.ownerId,
          role: 'OWNER'
        }
      }
    }

    return prisma.circle.create({ data })
  },

  /**
   * 更新圈子信息
   * @param id 圈子 ID
   * @param input 圈子更新参数
   * @returns 更新后的圈子记录
   */
  async update(id: number, input: UpdateCircleInput): Promise<Circle> {
    const data: Prisma.CircleUpdateInput = {
      name: input.name,
      description: input.description,
      cover: input.cover,
      themeColor: input.themeColor,
      isPublic: input.isPublic
    }

    Object.keys(data).forEach((key) => {
      if ((data as Record<string, unknown>)[key] === undefined) {
        delete (data as Record<string, unknown>)[key]
      }
    })

    return prisma.circle.update({
      where: { id },
      data
    })
  },

  /**
   * 分页查询圈子列表
   * @param options 查询选项
   * @returns 圈子列表与总数
   */
  async findMany(options: CircleListOptions = {}): Promise<{ list: Circle[]; total: number }> {
    const { page = 1, pageSize = 20, keyword, ownerId } = options
    const skip = (page - 1) * pageSize

    const where: Prisma.CircleWhereInput = {}

    if (keyword) {
      where.name = { contains: keyword }
    }

    if (ownerId) {
      where.ownerId = ownerId
    }

    const [list, total] = await Promise.all([
      prisma.circle.findMany({
        where,
        skip,
        take: pageSize,
        orderBy: { createdAt: 'desc' }
      }),
      prisma.circle.count({ where })
    ])

    return { list, total }
  },

  /**
   * 增加圈子成员数
   * @param id 圈子 ID
   * @param increment 增加数量（默认为 1）
   * @returns 更新后的圈子记录
   */
  async incrementMemberCount(id: number, increment: number = 1): Promise<Circle> {
    return prisma.circle.update({
      where: { id },
      data: { memberCount: { increment } }
    })
  },

  /**
   * 增加圈子帖子数
   * @param id 圈子 ID
   * @param increment 增加数量（默认为 1）
   * @returns 更新后的圈子记录
   */
  async incrementPostCount(id: number, increment: number = 1): Promise<Circle> {
    return prisma.circle.update({
      where: { id },
      data: { postCount: { increment } }
    })
  },

  /**
   * 查询用户加入的圈子列表
   * @param userId 用户 ID
   * @returns 圈子成员关系列表
   */
  async findUserCircles(userId: number) {
    return prisma.circleMember.findMany({
      where: { userId },
      include: {
        circle: true
      },
      orderBy: { createdAt: 'desc' }
    })
  },

  /**
   * 查询圈子通知的候选接收者，包含圈主并对成员 ID 去重。
   * @param circleId 圈子 ID
   * @returns 圈子不存在时返回 null，否则返回候选用户 ID
   */
  async findNotificationRecipientIds(circleId: number): Promise<number[] | null> {
    const circle = await prisma.circle.findUnique({
      where: { id: circleId },
      select: {
        ownerId: true,
        members: {
          select: { userId: true }
        }
      }
    })
    if (!circle) {
      return null
    }

    const userIds: number[] = [circle.ownerId]
    circle.members.forEach(function (membership) {
      if (userIds.indexOf(membership.userId) < 0) {
        userIds.push(membership.userId)
      }
    })
    return userIds
  },

  /**
   * 查询用户是否为圈子成员
   * @param circleId 圈子 ID
   * @param userId 用户 ID
   * @returns 成员关系或 null
   */
  async findMembership(circleId: number, userId: number) {
    return prisma.circleMember.findUnique({
      where: {
        userId_circleId: {
          userId,
          circleId
        }
      }
    })
  },

  /**
   * 添加圈子成员
   * @param circleId 圈子 ID
   * @param userId 用户 ID
   * @param role 成员角色
   * @returns 创建的成员关系
   */
  async addMember(circleId: number, userId: number, role: string = 'MEMBER') {
    return prisma.circleMember.create({
      data: {
        circleId,
        userId,
        role: role as CircleMemberRole
      }
    })
  },

  /**
   * 移除圈子成员
   * @param circleId 圈子 ID
   * @param userId 用户 ID
   * @returns 删除的成员关系
   */
  async removeMember(circleId: number, userId: number) {
    return prisma.circleMember.delete({
      where: {
        userId_circleId: {
          userId,
          circleId
        }
      }
    })
  },

  /**
   * 删除圈子
   * 关联的成员、帖子等会通过数据库级联删除处理。
   * @param id 圈子 ID
   * @returns 删除的圈子记录
   */
  async delete(id: number): Promise<Circle> {
    return prisma.circle.delete({
      where: { id }
    })
  },

  /**
   * 更新圈子成员角色
   * @param circleId 圈子 ID
   * @param userId 用户 ID
   * @param role 目标角色
   * @returns 更新后的成员关系
   */
  async updateMemberRole(circleId: number, userId: number, role: CircleMemberRole) {
    return prisma.circleMember.update({
      where: {
        userId_circleId: {
          userId,
          circleId
        }
      },
      data: { role }
    })
  },

  /**
   * 转让圈子拥有权
   * 将圈子 ownerId 指向新用户，并将新用户角色设为 OWNER；
   * 原拥有者降级为普通成员。
   * @param circleId 圈子 ID
   * @param fromUserId 原拥有者用户 ID
   * @param toUserId 新拥有者用户 ID
   * @returns 更新后的圈子记录
   */
  async transferOwnership(circleId: number, fromUserId: number, toUserId: number): Promise<Circle> {
    const [, , circle] = await prisma.$transaction([
      prisma.circleMember.update({
        where: {
          userId_circleId: {
            userId: fromUserId,
            circleId
          }
        },
        data: { role: 'MEMBER' }
      }),
      prisma.circleMember.update({
        where: {
          userId_circleId: {
            userId: toUserId,
            circleId
          }
        },
        data: { role: 'OWNER' }
      }),
      prisma.circle.update({
        where: { id: circleId },
        data: { ownerId: toUserId }
      })
    ])

    return circle
  }
}
