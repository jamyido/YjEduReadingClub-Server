// Package scheduler 实现后端定时任务调度。
// 当前包含三个 AI 自动总结任务，均以「AI伴读」系统用户身份发帖并置顶：
//   - 每日 23:00（北京时间）：收集当天帖子，总结共性问题，发布到「打卡挑战」话题
//   - 每周日 23:30：收集过去 7 天帖子，总结周精华，发布到「本周精读」话题
//   - 每月最后一天 23:30：收集过去 30 天帖子，总结月精华，发布到「本月精读」话题
//
// 调度实现使用标准库 time.Ticker，每分钟检查一次是否命中调度时刻，
// 避免引入额外 cron 依赖。
package scheduler

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/pkg/ai"
	"yjedu-reading-club-server/internal/pkg/businessdate"
	"yjedu-reading-club-server/internal/repository"
)

// shanghaiLocation 是北京时区（UTC+8），用于调度判断与时段计算。
// 使用固定偏移避免依赖系统 zoneinfo 数据库，与 businessdate 包保持一致。
var shanghaiLocation = time.FixedZone("CST", 8*60*60)

// 三个任务的调度时刻（北京时间）。
// 时刻错开避免同时运行导致的 AI 限流与数据库压力。
const (
	dailyScheduleHour   = 23
	dailyScheduleMinute = 0

	weeklyScheduleHour   = 23
	weeklyScheduleMinute = 30

	monthlyScheduleHour   = 23
	monthlyScheduleMinute = 45
)

// digestPostMaxItems 限制每次 AI 总结收集的最大帖子数，避免超出 AI token 上限。
const digestPostMaxItems = 50

// digestPostContentMaxRunes 限制单条帖子正文在 AI 提示词中的最大字符数。
const digestPostContentMaxRunes = 300

// weeklyReadingTopicSlug 是「本周精读」话题的稳定 slug。
const weeklyReadingTopicSlug = "weekly-reading"

// monthlyReadingTopicSlug 是「本月精读」话题的稳定 slug。
const monthlyReadingTopicSlug = "monthly-reading"

// taskState 跟踪单个任务的最近一次运行时间，避免在同一调度周期内重复运行。
type taskState struct {
	mu      sync.Mutex
	lastRun time.Time
}

// tryAcquire 尝试获取运行权。若距上次运行不足 minInterval 则返回 false。
// 该方法用于防止 ticker 多次触发同一调度时刻导致任务重复执行。
func (s *taskState) tryAcquire(now time.Time, minInterval time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lastRun.IsZero() && now.Sub(s.lastRun) < minInterval {
		return false
	}
	s.lastRun = now
	return true
}

var (
	dailyState   = &taskState{}
	weeklyState  = &taskState{}
	monthlyState = &taskState{}
)

// Start 启动定时调度器，立即返回；调度协程在后台运行直到进程退出。
// 调度周期为 1 分钟，每分钟检查是否有任务到点需运行。
func Start() {
	go func() {
		log.Println("[scheduler] 定时任务调度器已启动（北京时间）")
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			<-ticker.C
			dispatch(time.Now().In(shanghaiLocation))
		}
	}()
}

// dispatch 检查当前时间是否命中任一任务调度时刻，命中则在独立协程中执行任务。
// 任务的执行时长可能超过 1 分钟，因此使用 taskState 防止重复触发。
func dispatch(now time.Time) {
	// 每日任务：每天指定时刻运行。
	if now.Hour() == dailyScheduleHour && now.Minute() == dailyScheduleMinute {
		if dailyState.tryAcquire(now, 2*time.Minute) {
			go RunDailyDigest()
		}
	}
	// 每周任务：周日指定时刻运行。
	if now.Weekday() == time.Sunday && now.Hour() == weeklyScheduleHour && now.Minute() == weeklyScheduleMinute {
		if weeklyState.tryAcquire(now, 2*time.Minute) {
			go RunWeeklyDigest()
		}
	}
	// 每月任务：月末指定时刻运行。
	if isLastDayOfMonth(now) && now.Hour() == monthlyScheduleHour && now.Minute() == monthlyScheduleMinute {
		if monthlyState.tryAcquire(now, 2*time.Minute) {
			go RunMonthlyDigest()
		}
	}
}

// isLastDayOfMonth 判断给定时间是否为所在月份的最后一天。
// 通过给当前日期加 1 天并检查结果是否为 1 号来判断。
func isLastDayOfMonth(t time.Time) bool {
	nextDay := t.AddDate(0, 0, 1)
	return nextDay.Day() == 1
}

// RunDailyDigest 执行每日共性问题总结任务。
// 流程：1) 获取 AI伴读 用户 ID；2) 查询所有圈子；3) 对每个圈子收集当日帖子，调用 AI 总结，发布置顶帖到打卡挑战话题。
// 导出以供管理端手动触发调试使用。
func RunDailyDigest() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduler.daily] 任务异常: %v", r)
		}
	}()
	log.Println("[scheduler.daily] 开始执行每日共性问题总结任务")

	db := database.Get()
	aiID, err := repository.UserRepo.FindOrCreateAIAssistant(db)
	if err != nil {
		log.Printf("[scheduler.daily] 获取 AI伴读 用户失败: %v", err)
		return
	}
	topic, err := repository.TopicRepo.FindBySlug(db, businessdate.CheckInTopicSlug)
	if err != nil || topic == nil {
		log.Printf("[scheduler.daily] 打卡挑战话题未配置: %v", err)
		return
	}

	now := time.Now().In(shanghaiLocation)
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, shanghaiLocation)
	endTime := now

	circleIDs, err := repository.CircleRepo.FindAllIDs(db)
	if err != nil {
		log.Printf("[scheduler.daily] 查询圈子列表失败: %v", err)
		return
	}
	for _, circleID := range circleIDs {
		generateDigestPost(circleID, aiID, topic.ID, startTime, endTime, "daily", "每日共性问题")
	}
	log.Println("[scheduler.daily] 每日共性问题总结任务完成")
}

// RunWeeklyDigest 执行每周精华总结任务。
// 流程类似每日任务，但覆盖过去 7 天，发布到本周精读话题。
// 导出以供管理端手动触发调试使用。
func RunWeeklyDigest() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduler.weekly] 任务异常: %v", r)
		}
	}()
	log.Println("[scheduler.weekly] 开始执行每周精华总结任务")

	db := database.Get()
	aiID, err := repository.UserRepo.FindOrCreateAIAssistant(db)
	if err != nil {
		log.Printf("[scheduler.weekly] 获取 AI伴读 用户失败: %v", err)
		return
	}
	topic, err := repository.TopicRepo.FindBySlug(db, weeklyReadingTopicSlug)
	if err != nil || topic == nil {
		log.Printf("[scheduler.weekly] 本周精读话题未配置: %v", err)
		return
	}

	now := time.Now().In(shanghaiLocation)
	endTime := now
	startTime := now.AddDate(0, 0, -7)

	circleIDs, err := repository.CircleRepo.FindAllIDs(db)
	if err != nil {
		log.Printf("[scheduler.weekly] 查询圈子列表失败: %v", err)
		return
	}
	for _, circleID := range circleIDs {
		generateDigestPost(circleID, aiID, topic.ID, startTime, endTime, "weekly", "本周精华")
	}
	log.Println("[scheduler.weekly] 每周精华总结任务完成")
}

// RunMonthlyDigest 执行每月精华总结任务。
// 流程类似每周任务，但覆盖过去 30 天，发布到本月精读话题。
// 导出以供管理端手动触发调试使用。
func RunMonthlyDigest() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduler.monthly] 任务异常: %v", r)
		}
	}()
	log.Println("[scheduler.monthly] 开始执行每月精华总结任务")

	db := database.Get()
	aiID, err := repository.UserRepo.FindOrCreateAIAssistant(db)
	if err != nil {
		log.Printf("[scheduler.monthly] 获取 AI伴读 用户失败: %v", err)
		return
	}
	topic, err := repository.TopicRepo.FindBySlug(db, monthlyReadingTopicSlug)
	if err != nil || topic == nil {
		log.Printf("[scheduler.monthly] 本月精读话题未配置: %v", err)
		return
	}

	now := time.Now().In(shanghaiLocation)
	endTime := now
	startTime := now.AddDate(0, 0, -30)

	circleIDs, err := repository.CircleRepo.FindAllIDs(db)
	if err != nil {
		log.Printf("[scheduler.monthly] 查询圈子列表失败: %v", err)
		return
	}
	for _, circleID := range circleIDs {
		generateDigestPost(circleID, aiID, topic.ID, startTime, endTime, "monthly", "本月精华")
	}
	log.Println("[scheduler.monthly] 每月精华总结任务完成")
}

// generateDigestPost 收集圈子帖子并生成 AI 总结置顶帖。
// digestType 取值 "daily" / "weekly" / "monthly"，决定 AI 提示词与总结风格。
// digestLabel 是日志与帖子标题中使用的中文标签。
// 流程：
//  1. 查询圈子信息与时段内帖子（排除 AI伴读 自身发的总结帖）
//  2. 若无帖子则跳过
//  3. 构造 AI 系统提示词与用户消息，调用 AI 生成总结
//  4. 取消该圈子该话题旧 AI 总结帖的置顶
//  5. 以 AI伴读 身份创建新的置顶总结帖
//  6. 圈子帖子计数 +1
func generateDigestPost(circleID, aiUserID, topicID int64, startTime, endTime time.Time, digestType, digestLabel string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduler.%s] 圈子 %d 总结生成异常: %v", digestType, circleID, r)
		}
	}()

	db := database.Get()
	circle, err := repository.CircleRepo.FindByID(db, circleID)
	if err != nil || circle == nil {
		log.Printf("[scheduler.%s] 圈子 %d 不存在: %v", digestType, circleID, err)
		return
	}

	posts, err := repository.PostRepo.FindCirclePostsByTimeRange(db, circleID, startTime, endTime, aiUserID, digestPostMaxItems)
	if err != nil {
		log.Printf("[scheduler.%s] 查询圈子 %d 帖子失败: %v", digestType, circleID, err)
		return
	}
	if len(posts) == 0 {
		log.Printf("[scheduler.%s] 圈子 %d (%s) 时段内无帖子，跳过", digestType, circleID, circle.Name)
		return
	}

	systemPrompt := buildDigestSystemPrompt(digestType, circle.Name)
	userMessage := buildDigestUserMessage(digestType, digestLabel, circle.Name, startTime, endTime, posts)

	aiReply, chatErr := ai.Chat(systemPrompt, []ai.ChatMessage{{
		Content:     userMessage,
		ContentType: "text",
		Role:        "user",
		Type:        "question",
	}}, strconv.FormatInt(aiUserID, 10))
	if chatErr != nil {
		log.Printf("[scheduler.%s] 圈子 %d AI 总结失败，使用兜底文案: %v", digestType, circleID, chatErr)
		aiReply = buildFallbackDigest(digestType, digestLabel, circle.Name, startTime, endTime, posts)
	}

	title := buildDigestTitle(digestType, digestLabel, endTime)

	// 取消该圈子该话题旧 AI 总结帖的置顶，保证每个圈子每个话题仅最新一条置顶。
	if err := repository.PostRepo.UnpinPostsByCircleAndTopic(db, circleID, topicID); err != nil {
		log.Printf("[scheduler.%s] 圈子 %d 取消旧置顶失败: %v", digestType, circleID, err)
	}

	post, err := repository.PostRepo.CreatePinnedPost(db, repository.CreatePinnedPostInput{
		AuthorID: aiUserID,
		CircleID: circleID,
		TopicID:  topicID,
		Title:    title,
		Content:  aiReply,
	})
	if err != nil {
		log.Printf("[scheduler.%s] 圈子 %d 创建总结帖失败: %v", digestType, circleID, err)
		return
	}

	// 圈子帖子计数 +1，保持列表页统计准确。
	if err := repository.CircleRepo.IncrementPostCount(db, circleID, 1); err != nil {
		log.Printf("[scheduler.%s] 圈子 %d 更新帖子计数失败: %v", digestType, circleID, err)
	}

	log.Printf("[scheduler.%s] 圈子 %d (%s) 总结帖已创建: postID=%d", digestType, circleID, circle.Name, post.ID)
}

// buildDigestSystemPrompt 构造 AI 总结帖的系统提示词。
// 根据 digestType 引导 AI 生成不同风格的总结：每日聚焦共性问题，周/月聚焦精华分享。
func buildDigestSystemPrompt(digestType, circleName string) string {
	var b strings.Builder
	b.WriteString("你是「嘉阅圈」圈子「" + circleName + "」的 AI 共读助手。")
	b.WriteString("你的任务是根据圈友发布的帖子，生成一份结构清晰、内容凝练的总结帖，以 AI伴读 身份发布在该圈子内。\n")
	b.WriteString("输出要求：\n")
	b.WriteString("1. 使用 Markdown 格式，包含小标题与列表；\n")
	b.WriteString("2. 不要复述原帖，要提炼观点、归纳共性、突出亮点；\n")
	b.WriteString("3. 语气温和、富有启发，结尾给出 1-2 个引导性问题；\n")
	b.WriteString("4. 不要输出与总结无关的寒暄或解释，直接给出总结内容；\n")
	b.WriteString("5. 不要提及作者的真实手机号或敏感信息；\n")
	switch digestType {
	case "daily":
		b.WriteString("6. 本次为每日总结，重点归纳圈友今日提出的共性问题、困惑与讨论焦点；\n")
		b.WriteString("7. 输出结构建议：今日概览 / 共性问题 / 思考与引导。\n")
	case "weekly":
		b.WriteString("6. 本次为每周精华总结，重点提炼本周圈友分享的高光观点与精彩讨论；\n")
		b.WriteString("7. 输出结构建议：本周亮点 / 精华摘录 / 共读回顾。\n")
	case "monthly":
		b.WriteString("6. 本次为每月精华总结，重点回顾本月圈友共读的整体脉络与代表性分享；\n")
		b.WriteString("7. 输出结构建议：本月概览 / 精华集锦 / 共读展望。\n")
	}
	return b.String()
}

// buildDigestUserMessage 构造 AI 总结帖的用户消息，包含时段与帖子列表。
// 单条帖子正文超过 digestPostContentMaxRunes 字符时截断，避免超出 AI token 上限。
func buildDigestUserMessage(digestType, digestLabel, circleName string, startTime, endTime time.Time, posts []repository.CirclePostDigestItem) string {
	var b strings.Builder
	b.WriteString(digestLabel + " 任务输入：\n")
	b.WriteString("圈子：" + circleName + "\n")
	b.WriteString("时段：" + startTime.Format("2006-01-02 15:04") + " 至 " + endTime.Format("2006-01-02 15:04") + "\n")
	b.WriteString("帖子数量：" + strconv.Itoa(len(posts)) + "\n\n")
	b.WriteString("帖子列表（按时间升序）：\n")
	for i, p := range posts {
		b.WriteString("---\n")
		b.WriteString("[" + strconv.Itoa(i+1) + "] 作者：" + p.Nickname + "\n")
		b.WriteString("时间：" + p.CreatedAt.In(shanghaiLocation).Format("2006-01-02 15:04") + "\n")
		if p.Title != nil && *p.Title != "" {
			b.WriteString("标题：" + *p.Title + "\n")
		}
		content := p.Content
		if len([]rune(content)) > digestPostContentMaxRunes {
			content = string([]rune(content)[:digestPostContentMaxRunes]) + "..."
		}
		b.WriteString("内容：" + content + "\n")
	}
	b.WriteString("\n请根据以上帖子生成总结帖正文。")
	return b.String()
}

// buildDigestTitle 构造总结帖标题，包含时段标签与具体日期。
// 每日标题用日期，每周标题用周起止区间，每月标题用年月。
func buildDigestTitle(digestType, digestLabel string, endTime time.Time) string {
	switch digestType {
	case "daily":
		return digestLabel + " · " + endTime.Format("2006-01-02")
	case "weekly":
		// 周日为一周结束，往前推 6 天作为本周起始。
		weekStart := endTime.AddDate(0, 0, -6)
		return digestLabel + " · " + weekStart.Format("2006-01-02") + " ~ " + endTime.Format("2006-01-02")
	case "monthly":
		return digestLabel + " · " + endTime.Format("2006-01")
	}
	return digestLabel
}

// buildFallbackDigest 在 AI 调用失败时生成兜底总结帖正文。
// 仅简单列出帖子标题与作者，保证总结帖仍能发布。
func buildFallbackDigest(digestType, digestLabel, circleName string, startTime, endTime time.Time, posts []repository.CirclePostDigestItem) string {
	var b strings.Builder
	b.WriteString("# " + digestLabel + "\n\n")
	b.WriteString("圈子：" + circleName + "\n\n")
	b.WriteString("时段：" + startTime.Format("2006-01-02") + " 至 " + endTime.Format("2006-01-02") + "\n\n")
	b.WriteString("圈友共发布 " + strconv.Itoa(len(posts)) + " 篇分享，AI 总结生成中，请稍后查看完整版。\n\n")
	b.WriteString("## 分享列表\n\n")
	for i, p := range posts {
		title := "(无标题)"
		if p.Title != nil && *p.Title != "" {
			title = *p.Title
		}
		b.WriteString(strconv.Itoa(i+1) + ". " + p.Nickname + "：" + title + "\n")
	}
	return b.String()
}
