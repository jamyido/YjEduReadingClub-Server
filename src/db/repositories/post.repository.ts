import { Prisma, Post, PostType } from '@prisma/client'
import prisma from '@/lib/prisma'

/**
 * 帖子创建输入参数
 */
export type CreatePostInput = {
  authorId: number
  circleId?: number
  topicId: number
  type?: string
  title?: string
  content: string
  linkUrl?: string
  medias?: Array<{
    type: string
    url: string
    sort?: number
  }>
}

/**
 * 帖子更新输入参数
 */
export type UpdatePostInput = Partial<{
  title: string
  content: string
  linkUrl: string
  status: number
  isPinned: boolean
  isEssence: boolean
}>

/**
 * 帖子列表查询选项
 */
export type PostListOptions = {
  page?: number
  pageSize?: number
  circleId?: number
  authorId?: number
  topicId?: number
  type?: string
  status?: number
}

/**
 * 圈子累计发帖天数排行榜查询选项。
 */
export type CirclePostDaysRankingOptions = {
  page?: number
  pageSize?: number
}

/**
 * 圈子累计发帖天数排行榜记录。
 */
export type CirclePostDaysRankingRecord = {
  userId: number
  nickname: string
  avatar: string | null
  cumulativePostDays: number
  postCount: number
  lastPostAt: Date
}

/** MySQL 原始聚合查询返回的数据结构。 */
type CirclePostDaysRankingRawRow = {
  user_id: number
  nickname: string
  avatar: string | null
  cumulative_post_days: bigint | number
  post_count: bigint | number
  last_post_at: Date
}

/** MySQL COUNT 查询返回的数据结构。 */
type CirclePostDaysRankingTotalRow = {
  total: bigint | number
}

/** 帖子写入所需的最小 Prisma 客户端，可接收普通客户端或事务客户端。 */
type PostWriteDatabase = Pick<Prisma.TransactionClient, 'post'>

/**
 * 帖子仓库
 *
 * 职责：
 * - 封装帖子、帖子媒体、点赞、评论的数据库操作
 * - 提供列表分页、详情查询、状态更新等能力
 */
export const PostRepository = {
  /**
   * 根据 ID 查询帖子
   * @param id 帖子 ID
   * @returns 帖子记录或 null
   */
  async findById(id: number): Promise<Post | null> {
    return prisma.post.findUnique({
      where: { id }
    })
  },

  /**
   * 根据 ID 查询帖子详情（包含作者、圈子、媒体、评论）
   * @param id 帖子 ID
   * @returns 帖子详情或 null
   */
  async findDetailById(id: number) {
    return prisma.post.findUnique({
      where: { id },
      include: {
        author: {
          select: { id: true, nickname: true, avatar: true }
        },
        circle: {
          select: { id: true, name: true, cover: true }
        },
        topic: true,
        medias: {
          orderBy: { sort: 'asc' }
        }
      }
    })
  },

  /**
   * 创建帖子
   * @param input 帖子创建参数
   * @returns 创建后的帖子记录
   */
  async create(
    input: CreatePostInput,
    database: PostWriteDatabase = prisma
  ) {
    const data: Prisma.PostCreateInput = {
      author: { connect: { id: input.authorId } },
      topic: { connect: { id: input.topicId } },
      type: input.type as PostType,
      title: input.title,
      content: input.content,
      linkUrl: input.linkUrl,
      medias: input.medias && input.medias.length > 0
        ? { create: input.medias }
        : undefined
    }

    if (input.circleId) {
      data.circle = { connect: { id: input.circleId } }
    }

    return database.post.create({
      data,
      include: { topic: true }
    })
  },

  /**
   * 更新帖子
   * @param id 帖子 ID
   * @param input 帖子更新参数
   * @returns 更新后的帖子记录
   */
  async update(id: number, input: UpdatePostInput): Promise<Post> {
    const data: Prisma.PostUpdateInput = {
      title: input.title,
      content: input.content,
      linkUrl: input.linkUrl,
      status: input.status,
      isPinned: input.isPinned,
      isEssence: input.isEssence
    }

    Object.keys(data).forEach((key) => {
      if ((data as Record<string, unknown>)[key] === undefined) {
        delete (data as Record<string, unknown>)[key]
      }
    })

    return prisma.post.update({
      where: { id },
      data
    })
  },

  /**
   * 删除帖子（软删除，将状态置为 1）
   * @param id 帖子 ID
   * @returns 更新后的帖子记录
   */
  async softDelete(id: number): Promise<Post> {
    return prisma.post.update({
      where: { id },
      data: { status: 1 }
    })
  },

  /**
   * 分页查询帖子列表
   * @param options 查询选项
   * @returns 帖子列表与总数
   */
  async findMany(options: PostListOptions = {}) {
    const {
      page = 1,
      pageSize = 20,
      circleId,
      authorId,
      topicId,
      type,
      status = 0
    } = options
    const skip = (page - 1) * pageSize

    const where: Prisma.PostWhereInput = { status }

    if (circleId) {
      where.circleId = circleId
    }

    if (authorId) {
      where.authorId = authorId
    }

    if (topicId) {
      where.topicId = topicId
    }

    if (type) {
      where.type = type as PostType
    }

    const [list, total] = await Promise.all([
      prisma.post.findMany({
        where,
        skip,
        take: pageSize,
        orderBy: [
          { isPinned: 'desc' },
          { createdAt: 'desc' }
        ],
        include: {
          author: {
            select: { id: true, nickname: true, avatar: true }
          },
          circle: {
            select: { id: true, name: true, cover: true }
          },
          topic: true,
          medias: {
            orderBy: { sort: 'asc' }
          }
        }
      }),
      prisma.post.count({ where })
    ])

    return { list, total }
  },

  /**
   * 分页查询圈子累计发帖天数排行榜。
   * 正常帖子按 Asia/Shanghai 自然日去重，同一天发布多帖只累计一天；
   * 同分时依次按最近发帖时间降序、用户 ID 升序形成稳定名次。
   * @param circleId 圈子 ID
   * @param options 分页选项
   * @returns 排行记录与参与排行的用户总数
   */
  async findCirclePostDaysRanking(
    circleId: number,
    options: CirclePostDaysRankingOptions = {}
  ): Promise<{ list: CirclePostDaysRankingRecord[]; total: number }> {
    const page = options.page || 1
    const pageSize = options.pageSize || 50
    const skip = (page - 1) * pageSize

    const [rows, totalRows] = await Promise.all([
      prisma.$queryRaw<CirclePostDaysRankingRawRow[]>(Prisma.sql`
        SELECT
          users.id AS user_id,
          users.nickname AS nickname,
          users.avatar AS avatar,
          ranking.cumulative_post_days AS cumulative_post_days,
          ranking.post_count AS post_count,
          ranking.last_post_at AS last_post_at
        FROM (
          SELECT
            posts.author_id AS author_id,
            COUNT(DISTINCT DATE(CONVERT_TZ(posts.created_at, '+00:00', '+08:00'))) AS cumulative_post_days,
            COUNT(*) AS post_count,
            MAX(posts.created_at) AS last_post_at
          FROM posts
          WHERE posts.circle_id = ${circleId}
            AND posts.status = 0
          GROUP BY posts.author_id
        ) AS ranking
        INNER JOIN users ON users.id = ranking.author_id
        ORDER BY
          ranking.cumulative_post_days DESC,
          ranking.last_post_at DESC,
          users.id ASC
        LIMIT ${pageSize}
        OFFSET ${skip}
      `),
      prisma.$queryRaw<CirclePostDaysRankingTotalRow[]>(Prisma.sql`
        SELECT COUNT(DISTINCT posts.author_id) AS total
        FROM posts
        WHERE posts.circle_id = ${circleId}
          AND posts.status = 0
      `)
    ])

    const list = rows.map(function (row) {
      return {
        userId: row.user_id,
        nickname: row.nickname,
        avatar: row.avatar,
        cumulativePostDays: Number(row.cumulative_post_days),
        postCount: Number(row.post_count),
        lastPostAt: row.last_post_at
      }
    })
    const total = totalRows.length > 0 ? Number(totalRows[0].total) : 0

    return { list, total }
  },

  /**
   * 增加帖子点赞数
   * @param id 帖子 ID
   * @param increment 增加数量（默认为 1）
   * @returns 更新后的帖子记录
   */
  async incrementLikeCount(id: number, increment: number = 1): Promise<Post> {
    return prisma.post.update({
      where: { id },
      data: { likeCount: { increment } }
    })
  },

  /**
   * 增加帖子评论数
   * @param id 帖子 ID
   * @param increment 增加数量（默认为 1）
   * @returns 更新后的帖子记录
   */
  async incrementCommentCount(id: number, increment: number = 1): Promise<Post> {
    return prisma.post.update({
      where: { id },
      data: { commentCount: { increment } }
    })
  },

  /**
   * 增加帖子转发意图计数。
   * 微信小程序不提供最终发送成功回执，因此这里只统计用户打开转发面板的行为。
   * @param id 帖子 ID
   * @param increment 增加数量（默认为 1）
   * @returns 更新后的帖子记录
   */
  async incrementShareCount(id: number, increment: number = 1): Promise<Post> {
    return prisma.post.update({
      where: { id },
      data: { shareCount: { increment } }
    })
  },

  /**
   * 批量查询用户已点赞的帖子 ID。
   * @param userId 用户 ID
   * @param postIds 帖子 ID 列表
   * @returns 已点赞的帖子 ID 列表
   */
  async findLikedPostIds(userId: number, postIds: number[]): Promise<number[]> {
    if (postIds.length === 0) {
      return []
    }

    const likes = await prisma.like.findMany({
      where: {
        userId,
        targetType: 'post',
        targetId: { in: postIds }
      },
      select: { targetId: true }
    })

    return likes.map(function (like) { return like.targetId })
  },

  /**
   * 查询用户是否点赞指定目标
   * @param userId 用户 ID
   * @param targetType 目标类型
   * @param targetId 目标 ID
   * @returns 点赞记录或 null
   */
  async findLike(userId: number, targetType: string, targetId: number) {
    return prisma.like.findUnique({
      where: {
        userId_targetType_targetId: {
          userId,
          targetType,
          targetId
        }
      }
    })
  },

  /**
   * 创建点赞记录
   * @param userId 用户 ID
   * @param targetType 目标类型
   * @param targetId 目标 ID
   * @returns 创建的点赞记录
   */
  async createLike(userId: number, targetType: string, targetId: number) {
    return prisma.like.create({
      data: {
        userId,
        targetType,
        targetId
      }
    })
  },

  /**
   * 删除点赞记录
   * @param userId 用户 ID
   * @param targetType 目标类型
   * @param targetId 目标 ID
   * @returns 删除的点赞记录
   */
  async deleteLike(userId: number, targetType: string, targetId: number) {
    return prisma.like.delete({
      where: {
        userId_targetType_targetId: {
          userId,
          targetType,
          targetId
        }
      }
    })
  },

  /**
   * 创建评论
   * @param input 评论参数
   * @returns 创建的评论记录
   */
  async createComment(input: {
    postId: number
    authorId: number
    content: string
    parentId?: number
    replyToId?: number
  }) {
    return prisma.comment.create({
      data: {
        postId: input.postId,
        authorId: input.authorId,
        content: input.content,
        parentId: input.parentId,
        replyToId: input.replyToId
      }
    })
  },

  /**
   * 查询评论详情
   * @param id 评论 ID
   * @returns 评论详情或 null
   */
  async findCommentById(id: number) {
    return prisma.comment.findUnique({
      where: { id },
      include: {
        author: {
          select: { id: true, nickname: true, avatar: true }
        }
      }
    })
  }
}
