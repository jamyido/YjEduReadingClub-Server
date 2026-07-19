package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/config"
	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/pkg/response"
)

// uploadExtensionMap 将允许的图片 MIME 类型映射为可信扩展名。
// 避免直接使用客户端文件名造成路径或格式风险。
var uploadExtensionMap = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// validUploadKinds 是合法的上传用途集合。
var validUploadKinds = map[string]bool{
	"avatar": true,
	"circle": true,
	"post":   true,
}

// resolveUploadKind 归一化上传用途参数，默认 avatar。
func resolveUploadKind(raw string) (string, bool) {
	if raw == "" {
		return "avatar", true
	}
	if validUploadKinds[raw] {
		return raw, true
	}
	return "", false
}

// generateUploadFileName 生成 {kind}_{timestamp}_{random8}{ext} 形式的唯一文件名。
func generateUploadFileName(kind, ext string) string {
	timestamp := time.Now().UnixMilli()
	randBuf := make([]byte, 4)
	_, _ = rand.Read(randBuf)
	return fmt.Sprintf("%s_%d_%s%s", kind, timestamp, hex.EncodeToString(randBuf), ext)
}

// resolveUploadSubDir 根据上传用途返回相对 uploads 根目录的子目录路径。
// avatar 走 users/{userId}/avatar/ —— 按用户分组便于服务器整理维护；
// circle 走 circles/circle_{circleId}/ —— 按圈子分组，加 circle_ 前缀避免与用户 ID 冲突；
// 其他用途沿用 {userId}/ 保持兼容。
func resolveUploadSubDir(kind string, userID int64, circleID int64) string {
	if kind == "avatar" {
		return fmt.Sprintf("users/%d/avatar", userID)
	}
	if kind == "circle" {
		// 创建圈子时还没有 circleId，临时存到用户目录下的 circle_temp 子目录。
		// 创建成功后由业务层决定是否迁移；更新圈子封面时必须传 circleId。
		if circleID <= 0 {
			return fmt.Sprintf("users/%d/circle_temp", userID)
		}
		return fmt.Sprintf("circles/circle_%d", circleID)
	}
	return fmt.Sprintf("%d", userID)
}

// resolveUploadRoot 推导上传根目录的绝对路径。
// 优先使用配置中的 UploadDir（可为绝对或相对路径），缺失时回退到工作目录下的 uploads。
func resolveUploadRoot() string {
	cfg := config.Get()
	dir := "uploads"
	if cfg != nil && cfg.UploadDir != "" {
		dir = cfg.UploadDir
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return dir
	}
	return filepath.Join(cwd, dir)
}

// maxUploadBytes 返回单文件上传字节数上限。
func maxUploadBytes() int64 {
	cfg := config.Get()
	if cfg != nil && cfg.MaxUploadSizeMB > 0 {
		return cfg.MaxUploadSizeMB * 1024 * 1024
	}
	return 10 * 1024 * 1024
}

// sniffImageMimeType 通过读取文件头部字节判断图片 MIME 类型。
// 仅识别 JPG/PNG/WebP/GIF 四种允许的格式，避免信任客户端 Content-Type。
func sniffImageMimeType(fileHeader *multipart.FileHeader) string {
	file, err := fileHeader.Open()
	if err != nil {
		return ""
	}
	defer file.Close()

	header := make([]byte, 12)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return ""
	}
	header = header[:n]

	// JPG: FF D8 FF
	if len(header) >= 3 && header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF {
		return "image/jpeg"
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(header) >= 8 && bytes.Equal(header[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	// GIF: 47 49 46 38 (37 a / 39 a)
	if len(header) >= 6 && bytes.Equal(header[:6], []byte("GIF87a")) || bytes.Equal(header[:6], []byte("GIF89a")) {
		return "image/gif"
	}
	// WebP: RIFF .... WEBP
	if len(header) >= 12 && bytes.Equal(header[:4], []byte("RIFF")) && bytes.Equal(header[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	return ""
}

// UploadFile 处理 POST /api/upload。
// 接收 multipart/form-data 文件上传，按用户 ID / 圈子 ID 分目录保存。
// 查询参数：
//   - kind: 上传用途（avatar/circle/post），默认 avatar
//   - circleId: 圈子 ID（kind=circle 且更新已有圈子封面时必传）
// @Summary      文件上传
// @Description  需登录。接收 multipart/form-data 文件上传，按用户 ID / 圈子 ID 分目录保存；仅支持 JPG、PNG、WebP、GIF 图片，单文件不超过 10MB。
// @Tags         上传
// @Accept       multipart/form-data
// @Produce      json
// @Param        file      formData  file    true   "待上传文件"
// @Param        kind      query     string  false  "上传用途（avatar/circle/post），默认 avatar"  default(avatar)
// @Param        circleId  query     int64   false  "圈子 ID（kind=circle 且更新已有圈子封面时必传）"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "未接收到上传文件或文件过大或格式不支持"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      500  {object}  response.ApiResponse  "保存文件失败"
// @Router       /upload [post]
func UploadFile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	kind, ok := resolveUploadKind(strings.TrimSpace(c.Query("kind")))
	if !ok {
		response.SendError(c, 400, "INVALID_UPLOAD_KIND", "上传图片用途无效")
		return
	}

	// kind=circle 时解析可选的 circleId 查询参数，用于按圈子分目录归档。
	var circleID int64
	if kind == "circle" {
		circleID = parseInt64Query(c, "circleId")
	}

	// 限制请求体大小，防止超大上传耗尽内存。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes()+1024)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.SendError(c, 400, "UPLOAD_FAILED", "未接收到上传文件或文件过大")
		return
	}

	// 通过文件头嗅探真实 MIME，避免信任客户端 Content-Type。
	mimeType := sniffImageMimeType(fileHeader)
	if mimeType == "" {
		response.SendError(c, 400, "INVALID_FILE_TYPE", "仅支持图片上传")
		return
	}
	ext, ok := uploadExtensionMap[mimeType]
	if !ok {
		response.SendError(c, 400, "UNSUPPORTED_IMAGE_TYPE", "仅支持 JPG、PNG、WebP 或 GIF 图片")
		return
	}

	if fileHeader.Size == 0 {
		response.SendError(c, 400, "EMPTY_FILE", "上传文件不能为空")
		return
	}
	if fileHeader.Size > maxUploadBytes() {
		response.SendError(c, 400, "FILE_TOO_LARGE", "文件大小不能超过 10MB")
		return
	}

	fileName := generateUploadFileName(kind, ext)
	subDir := resolveUploadSubDir(kind, user.ID, circleID)
	userDir := filepath.Join(resolveUploadRoot(), subDir)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		response.SendError(c, 500, "UPLOAD_FAILED", "创建上传目录失败")
		return
	}

	dst := filepath.Join(userDir, fileName)
	if err := c.SaveUploadedFile(fileHeader, dst); err != nil {
		response.SendError(c, 500, "UPLOAD_FAILED", "保存文件失败")
		return
	}

	url := fmt.Sprintf("/uploads/%s/%s", subDir, fileName)
	response.SendSuccess(c, gin.H{
		"url":    url,
		"userId": user.ID,
		"kind":   kind,
	}, "上传成功")
}
