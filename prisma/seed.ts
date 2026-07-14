import { PrismaClient, Gender, UserRole, UserStatus } from '@prisma/client'
import bcrypt from 'bcryptjs'

const prisma = new PrismaClient()

/**
 * 管理员默认密码
 */
const ADMIN_PASSWORD = '123456'

/**
 * 管理员默认手机号
 */
const ADMIN_PHONE = '13800000000'

/**
 * 对明文密码进行 bcrypt 哈希
 * @param plainPassword 明文密码
 * @returns 哈希后的密码
 */
async function hashPassword(plainPassword: string): Promise<string> {
  return bcrypt.hash(plainPassword, 10)
}

/**
 * 数据库种子脚本
 *
 * 职责：
 * - 以幂等 upsert 方式确保系统管理员存在
 * - 以稳定 slug 创建或更新系统话题
 * - 不删除、不重置任何现有业务数据
 */
async function main() {
  console.log('开始同步系统种子数据...')

  const hashedPassword = await hashPassword(ADMIN_PASSWORD)

  await prisma.user.upsert({
    where: { phone: ADMIN_PHONE },
    create: {
        phone: ADMIN_PHONE,
        password: hashedPassword,
        nickname: '嘉阅管理员',
        avatar: '/assets/avatars/avatar-studio.png',
        bio: '嘉阅圈官方管理员账号',
        gender: Gender.UNKNOWN,
        role: UserRole.ADMIN,
        status: UserStatus.ACTIVE
    },
    update: {
      role: UserRole.ADMIN,
      status: UserStatus.ACTIVE
    }
  })

  const topics = [
    {
      slug: 'weekly-reading',
      title: '本周精读',
      description: '拆解一本书的关键观点',
      status: 1,
      sort: 30
    },
    {
      slug: 'check-in-challenge',
      title: '打卡挑战',
      description: '记录每天的阅读与成长',
      status: 1,
      sort: 100
    },
    {
      slug: 'course-resources',
      title: '课程资料',
      description: '文档、视频、回放统一收纳',
      status: 1,
      sort: 20
    }
  ]

  for (const topic of topics) {
    await prisma.topic.upsert({
      where: { slug: topic.slug },
      create: topic,
      update: {
        title: topic.title,
        description: topic.description,
        status: topic.status,
        sort: topic.sort
      }
    })
  }

  console.log('系统管理员与话题同步完成')
}

main()
  .catch((error) => {
    console.error('种子数据创建失败:', error)
    process.exit(1)
  })
  .finally(async () => {
    await prisma.$disconnect()
  })
