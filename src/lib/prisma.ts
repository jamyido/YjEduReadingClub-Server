import { PrismaClient } from '@prisma/client'

/**
 * 全局 Prisma Client 类型声明
 * 用于在开发环境下保持单例，避免热更新时创建多个连接实例
 */
const globalForPrisma = global as unknown as {
  prisma: PrismaClient | undefined
}

/**
 * Prisma Client 单例
 *
 * 职责：
 * - 提供整个后端唯一的数据库访问入口
 * - 开发环境热更新时复用已有实例，防止连接泄漏
 * - 生产环境每次启动创建新实例
 */
export const prisma = globalForPrisma.prisma || new PrismaClient()

if (process.env.NODE_ENV !== 'production') {
  globalForPrisma.prisma = prisma
}

export default prisma
