import { Prisma, Topic } from '@prisma/client'
import prisma from '@/lib/prisma'

/** 话题列表查询选项。 */
export type TopicListOptions = {
  page?: number
  pageSize?: number
  query?: string
  status?: number
}

/**
 * 话题仓库。
 * 负责话题读取、搜索与帖子数量聚合；系统话题由 migration/seed 管理。
 */
export const TopicRepository = {
  /**
   * 根据 ID 查询话题。
   * @param id 话题 ID
   * @returns 话题或 null
   */
  async findById(id: number): Promise<Topic | null> {
    return prisma.topic.findUnique({ where: { id } })
  },

  /**
   * 根据稳定 slug 查询话题。
   * @param slug 话题业务标识
   * @returns 话题或 null
   */
  async findBySlug(slug: string): Promise<Topic | null> {
    return prisma.topic.findUnique({ where: { slug } })
  },

  /**
   * 分页查询启用话题，并统计每个话题的正常帖子数量。
   * @param options 分页、搜索与状态过滤条件
   * @returns 话题列表及总数
   */
  async findMany(options: TopicListOptions = {}) {
    const page = options.page || 1
    const pageSize = options.pageSize || 20
    const status = options.status === undefined ? 1 : options.status
    const query = options.query ? options.query.trim() : ''
    const skip = (page - 1) * pageSize

    const where: Prisma.TopicWhereInput = { status }
    if (query) {
      where.OR = [
        { title: { contains: query } },
        { description: { contains: query } }
      ]
    }

    const listPromise = prisma.topic.findMany({
      where,
      skip,
      take: pageSize,
      orderBy: [
        { sort: 'desc' },
        { createdAt: 'asc' }
      ],
      include: {
        _count: {
          select: {
            posts: {
              where: { status: 0 }
            }
          }
        }
      }
    })

    const countPromise = prisma.topic.count({ where })
    const result = await Promise.all([listPromise, countPromise])

    return { list: result[0], total: result[1] }
  }
}
