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
  // 静态文件服务：/uploads/* 映射到 server/public/uploads/*
  // 用于头像等用户上传文件
}

module.exports = nextConfig
