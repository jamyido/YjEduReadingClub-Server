/**
 * @type {import('next').NextConfig}
 */
const nextConfig = {
  reactStrictMode: true,
  swcMinify: true,
  // 后端服务关闭 React 服务端组件，统一使用 API Route 提供数据接口
  experimental: {
    serverComponentsExternalPackages: ['@prisma/client', 'bcryptjs']
  },
  // 上传文件访问：对外保持 /uploads/{userId}/{filename} 的 URL 不变，
  // 内部 rewrite 到 API 路由 /api/uploads/{userId}/{filename} 由
  // pages/api/uploads/[...path].ts 读取磁盘文件流式返回。
  // 使用 beforeFiles 阶段 rewrite，确保所有 /uploads/* 请求都由该
  // API 路由处理（避免 public/uploads/ 静态文件缓存问题），并支持
  // 服务运行期新增文件无需重启即可访问。
  async rewrites() {
    return {
      beforeFiles: [
        {
          source: '/uploads/:path*',
          destination: '/api/uploads/:path*'
        }
      ]
    }
  }
}

module.exports = nextConfig
