// Package main 是后端服务入口，负责加载配置、初始化数据库并启动 HTTP 服务。
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/config"
	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/pkg/ai"
	"yjedu-reading-club-server/internal/router"
	"yjedu-reading-club-server/internal/scheduler"

	// 引入 swag 自动生成的 docs 包，使 /swagger/* 能加载 OpenAPI 文档。
	_ "yjedu-reading-club-server/docs"
)

// @title           嘉阅圈后端 API
// @version         1.0
// @description     嘉阅圈私域社群平台后端服务，提供内容社区、互动打卡、小组交流、私信通知、知识课程等核心能力。
// @description     统一响应格式：{"success":bool,"data":any,"message":string,"error":{"code":string,"message":string}}
// @host            localhost:3100
// @BasePath        /api
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     输入 "Bearer <token>"，token 通过 /api/auth/login/phone 或 /api/auth/weapp/login 获取。

// main 是程序入口。
// 启动顺序：解析启动参数 -> 加载配置 -> 初始化数据库 -> 装配路由 -> 启动 HTTP 服务。
// 启动参数：--env=development|production，默认 development。
// 微信小程序集成通过 config.Get() 读取配置，无需显式初始化。
func main() {
	// 1. 解析 --env 启动参数（development / production）。
	env := config.ParseEnvFromFlags()

	// 2. 按环境加载配置（.env.{env} 文件 + 系统环境变量）。
	cfg := config.Load(env)
	log.Printf("运行环境：%s\n", cfg.Env)

	// 3. 按环境设置 Gin 运行模式：开发 debug / 生产 release。
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	if cfg.JWTSecret == "" {
		log.Println("警告：JWT_SECRET 未配置，登录功能将不可用")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL 未配置，服务无法启动")
	}

	// 4. 初始化数据库连接。
	db, err := database.Init()
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 5. 自动迁移表结构并在数据库为空时写入种子数据（管理员账户与 6 个初始圈子）。
	if err := database.Initialize(db); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 5.1 根据 AI_PROVIDER 配置初始化 AI provider（coze / deepseek）。
	selectedAI := ai.SelectProvider()
	log.Printf("AI provider 已选择：%s\n", selectedAI.Name())

	// 5.2 启动定时任务调度器（每日/每周/每月 AI 总结帖）。
	scheduler.Start()

	// 6. 装配路由。
	engine := router.Setup()

	// 7. 启动 HTTP 服务，支持优雅关闭。
	port := cfg.Port
	if port == "" {
		port = "3001"
	}
	addr := fmt.Sprintf(":%s", port)
	server := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 启动监听协程。
	go func() {
		log.Printf("嘉阅圈后端服务启动于 http://localhost%s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务启动失败: %v", err)
		}
	}()

	// 等待中断信号，优雅关闭。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("服务关闭异常: %v", err)
	}
	if db, err := database.Get().DB(); err == nil {
		_ = db.Close()
	}
	log.Println("服务已退出")
}
