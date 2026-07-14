import { execSync } from 'child_process'

/**
 * 数据库初始化脚本
 *
 * 职责：
 * - 在生产环境一键执行 Prisma Migrate Deploy 与 Seed
 * - 部署到 Ubuntu 私云时可通过 `yarn init-db` 调用
 */
function main() {
  console.log('开始执行数据库迁移...')
  execSync('npx prisma migrate deploy', {
    stdio: 'inherit',
    cwd: process.cwd()
  })

  console.log('开始写入种子数据...')
  execSync('npx prisma db seed', {
    stdio: 'inherit',
    cwd: process.cwd()
  })

  console.log('数据库初始化完成')
}

main()
