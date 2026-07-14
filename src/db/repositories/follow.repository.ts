import { Prisma, Follow } from '@prisma/client'
import prisma from '@/lib/prisma'

/**
 * 关注关系仓库
 *
 * 职责：
 * - 封装用户关注/取关的数据库操作
 * - 提供关注列表与粉丝列表查询
 */
export const FollowRepository = {
  /**
   * 查询关注关系是否存在
   * @param followerId 关注者 ID
   * @param followingId 被关注者 ID
   * @returns 关注记录或 null
   */
  async findRelation(followerId: number, followingId: number): Promise<Follow | null> {
    return prisma.follow.findUnique({
      where: {
        followerId_followingId: { followerId, followingId }
      }
    })
  },

  /**
   * 创建关注关系
   * @param followerId 关注者 ID
   * @param followingId 被关注者 ID
   * @returns 创建的关注记录
   */
  async follow(followerId: number, followingId: number): Promise<Follow> {
    return prisma.follow.create({
      data: { followerId, followingId }
    })
  },

  /**
   * 删除关注关系
   * @param followerId 关注者 ID
   * @param followingId 被关注者 ID
   * @returns 删除的关注记录
   */
  async unfollow(followerId: number, followingId: number): Promise<Follow> {
    return prisma.follow.delete({
      where: {
        followerId_followingId: { followerId, followingId }
      }
    })
  },

  /**
   * 查询用户关注列表（分页）
   * @param userId 用户 ID
   * @param page 页码
   * @param pageSize 每页数量
   * @returns 关注列表与总数
   */
  async findFollowing(
    userId: number,
    page: number = 1,
    pageSize: number = 20
  ): Promise<{ list: Follow[]; total: number }> {
    const skip = (page - 1) * pageSize
    const where: Prisma.FollowWhereInput = { followerId: userId }

    const [list, total] = await Promise.all([
      prisma.follow.findMany({
        where,
        skip,
        take: pageSize,
        include: {
          following: {
            select: { id: true, nickname: true, avatar: true, bio: true }
          }
        },
        orderBy: { createdAt: 'desc' }
      }),
      prisma.follow.count({ where })
    ])

    return { list, total }
  },

  /**
   * 查询用户粉丝列表（分页）
   * @param userId 用户 ID
   * @param page 页码
   * @param pageSize 每页数量
   * @returns 粉丝列表与总数
   */
  async findFollowers(
    userId: number,
    page: number = 1,
    pageSize: number = 20
  ): Promise<{ list: Follow[]; total: number }> {
    const skip = (page - 1) * pageSize
    const where: Prisma.FollowWhereInput = { followingId: userId }

    const [list, total] = await Promise.all([
      prisma.follow.findMany({
        where,
        skip,
        take: pageSize,
        include: {
          follower: {
            select: { id: true, nickname: true, avatar: true, bio: true }
          }
        },
        orderBy: { createdAt: 'desc' }
      }),
      prisma.follow.count({ where })
    ])

    return { list, total }
  }
}
