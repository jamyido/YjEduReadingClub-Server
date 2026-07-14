import { Prisma, Comment } from '@prisma/client'
import prisma from '@/lib/prisma'

/**
 * 评论仓库
 *
 * 职责：
 * - 封装评论的创建与查询
 * - 提供一级评论列表与二级回复查询
 */
export const CommentRepository = {
  /**
   * 创建评论
   * @param input 评论参数
   * @returns 创建的评论记录
   */
  async create(input: {
    postId: number
    authorId: number
    content: string
    parentId?: number
    replyToId?: number
  }): Promise<Comment> {
    return prisma.comment.create({
      data: {
        postId: input.postId,
        authorId: input.authorId,
        content: input.content,
        parentId: input.parentId,
        replyToId: input.replyToId
      },
      include: {
        author: {
          select: { id: true, nickname: true, avatar: true }
        }
      }
    })
  },

  /**
   * 根据 ID 查询评论
   * @param id 评论 ID
   * @returns 评论记录或 null
   */
  async findById(id: number): Promise<Comment | null> {
    return prisma.comment.findUnique({
      where: { id },
      include: {
        author: {
          select: { id: true, nickname: true, avatar: true }
        }
      }
    })
  },

  /**
   * 判断用户是否为指定一级评论线程中的有效参与者。
   * @param parentId 一级评论 ID
   * @param userId 待校验用户 ID
   * @returns 用户是一级评论作者或有效回复作者时返回 true
   */
  async isActiveThreadParticipant(parentId: number, userId: number): Promise<boolean> {
    const count = await prisma.comment.count({
      where: {
        authorId: userId,
        status: 0,
        OR: [
          { id: parentId, parentId: null },
          { parentId }
        ]
      }
    })
    return count > 0
  },

  /**
   * 查询帖子的一级评论列表（分页，附带二级回复）
   * @param postId 帖子 ID
   * @param page 页码
   * @param pageSize 每页数量
   * @returns 评论列表与总数
   */
  async findByPost(
    postId: number,
    page: number = 1,
    pageSize: number = 20
  ): Promise<{ list: Comment[]; total: number }> {
    const skip = (page - 1) * pageSize
    const where: Prisma.CommentWhereInput = {
      postId,
      parentId: null,
      status: 0
    }

    const [list, total] = await Promise.all([
      prisma.comment.findMany({
        where,
        skip,
        take: pageSize,
        include: {
          author: {
            select: { id: true, nickname: true, avatar: true }
          },
          replies: {
            include: {
              author: {
                select: { id: true, nickname: true, avatar: true }
              }
            },
            orderBy: { createdAt: 'asc' }
          }
        },
        orderBy: { createdAt: 'desc' }
      }),
      prisma.comment.count({ where })
    ])

    return { list, total }
  },

  /**
   * 软删除评论（将状态置为 1）
   * @param id 评论 ID
   * @returns 更新后的评论记录
   */
  async softDelete(id: number): Promise<Comment> {
    return prisma.comment.update({
      where: { id },
      data: { status: 1 }
    })
  }
}
