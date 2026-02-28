package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdk "github.com/matrixorigin/moi-go-sdk"
)

type AIService struct {
	baseURL     string
	apiKey      string
	model       string
	fastModel   string
	dbName      string
	catalogDBID int
	client      *http.Client
	raw         *sdk.RawClient
}

func NewAIService(baseURL, apiKey, model, fastModel, dbName string, raw *sdk.RawClient) *AIService {
	return &AIService{baseURL: baseURL, apiKey: apiKey, model: model, fastModel: fastModel, dbName: dbName, client: &http.Client{}, raw: raw}
}

func (s *AIService) SetCatalogDBID(id int) { s.catalogDBID = id }

func (s *AIService) doChat(ctx context.Context, system, user string, stream bool, flush func(string)) (string, error) {
	return s.doChatWithModel(ctx, s.model, system, user, stream, flush)
}

func (s *AIService) doChatWithModel(ctx context.Context, model, system, user string, stream bool, flush func(string)) (string, error) {
	return s.doChatWithHistory(ctx, model, system, nil, user, stream, flush)
}

func (s *AIService) doChatWithHistory(ctx context.Context, model, system string, history []map[string]string, user string, stream bool, flush func(string)) (string, error) {
	msgs := []map[string]string{{"role": "system", "content": system}}
	msgs = append(msgs, history...)
	msgs = append(msgs, map[string]string{"role": "user", "content": user})

	body := map[string]interface{}{
		"model":    model,
		"stream":   stream,
		"messages": msgs,
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/llm-proxy/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("moi-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, data)
	}

	if !stream {
		data, _ := io.ReadAll(resp.Body)
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}
		if len(result.Choices) == 0 {
			return "", fmt.Errorf("empty choices")
		}
		return result.Choices[0].Message.Content, nil
	}

	scanner := bufio.NewScanner(resp.Body)
	var full strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
			token := chunk.Choices[0].Delta.Content
			if token != "" {
				full.WriteString(token)
				if flush != nil {
					flush(token)
				}
			}
		}
	}
	return full.String(), nil
}

func (s *AIService) chat(ctx context.Context, system, user string) (string, error) {
	return s.doChat(ctx, system, user, false, nil)
}

func (s *AIService) stream(ctx context.Context, system, user string, flush func(string)) (string, error) {
	return s.doChat(ctx, system, user, true, flush)
}

// ValidateWorkContent 快速判断输入是否为有效工作内容
func (s *AIService) ValidateWorkContent(ctx context.Context, content string) (valid bool, reply string, err error) {
	system := `判断用户输入是否为"今天做了什么"的工作汇报内容。
有效：完成了某任务、修复了某bug、开了某会议、写了某文档等具体工作事项的陈述。
无效：提问、请求建议、闲聊、感想、抱怨（如"怎么办"、"你觉得呢"、"如何解决"）。
返回 JSON：{"valid":true} 或 {"valid":false,"reply":"友好引导语"}。只返回 JSON。`

	result, err := s.doChatWithModel(ctx, s.fastModel, system, content, false, nil)
	if err != nil {
		return true, "", fmt.Errorf("validate: %w", err)
	}
	var parsed struct {
		Valid bool   `json:"valid"`
		Reply string `json:"reply"`
	}
	if json.Unmarshal([]byte(result), &parsed) != nil {
		return true, "", nil
	}
	return parsed.Valid, parsed.Reply, nil
}

// ExtractWorkContent 从对话历史中提取完整的工作内容（让 LLM 理解上下文）
func (s *AIService) ExtractWorkContent(ctx context.Context, current string, history []map[string]string) (string, error) {
	// 只保留用户消息，避免把 AI 编造的内容当成用户说的
	var userHistory []map[string]string
	for _, m := range history {
		if m["role"] == "user" {
			userHistory = append(userHistory, m)
		}
	}
	if len(userHistory) == 0 {
		return current, nil
	}
	system := `你是日报助手。用户在多轮对话中描述了工作内容，请从用户的历史消息和最新消息中提取完整的工作描述。
规则：
- 只提取用户明确说过的内容，绝对不要添加用户没说过的细节
- 合并相关信息为完整描述
- 只输出提取后的工作内容文本，不加任何解释`
	result, err := s.doChatWithHistory(ctx, s.model, system, userHistory, current, false, nil)
	if err != nil {
		return current, err
	}
	return strings.TrimSpace(result), nil
}

// AssessCompleteness 判断工作内容是否足够详细，不够则返回追问
func (s *AIService) AssessCompleteness(ctx context.Context, content string) (sufficient bool, followUp string, err error) {
	// 兜底：内容太短（去掉标点后不足6字）直接追问
	clean := strings.TrimSpace(content)
	for _, c := range []string{"。", "，", ".", ",", "!", "！", "?", "？"} {
		clean = strings.ReplaceAll(clean, c, "")
	}
	if len([]rune(clean)) < 6 {
		return false, "能再具体说说吗？比如做了什么、涉及哪个模块？", nil
	}

	system := `你是日报审核员。判断用户的工作描述是否足够具体，能形成一条有意义的日报。

【不通过】没说清楚具体做了什么：
- "登录的" "登录模块" → 不通过（登录模块怎么了？做了什么？）
- "修了个bug" → 不通过（什么bug？）
- "写了代码" → 不通过
- "做了点优化" → 不通过

【通过】说清楚了做了什么事+涉及什么：
- "修复了登录验证码的bug" → 通过（有动作+有对象）
- "完成用户管理接口开发" → 通过
- "参加了产品评审会" → 通过

返回 JSON：{"sufficient":true} 或 {"sufficient":false,"followUp":"简短追问（一句话）"}。只返回 JSON。`

	result, err := s.doChatWithModel(ctx, s.fastModel, system, content, false, nil)
	if err != nil {
		return true, "", err
	}
	var parsed struct {
		Sufficient bool   `json:"sufficient"`
		FollowUp   string `json:"followUp"`
	}
	if json.Unmarshal([]byte(result), &parsed) != nil {
		return true, "", nil
	}
	return parsed.Sufficient, parsed.FollowUp, nil
}

// StreamSummarize 流式生成工作摘要（纯文本，每条 • 开头）
func (s *AIService) StreamSummarize(ctx context.Context, content string, flush func(string)) (string, error) {
	system := `你是日报摘要助手。用简洁要点总结用户的工作内容。规则：
- 每条以 • 开头，每条独占一行
- 同一件事的描述和补充信息合并为一条，不要拆分
- 直接输出摘要文本，不要加标题或额外说明`
	result, err := s.stream(ctx, system, content, flush)
	if err != nil {
		return "", fmt.Errorf("stream summarize: %w", err)
	}
	return result, nil
}

// DetectRisks 从摘要中检测风险项
func (s *AIService) DetectRisks(ctx context.Context, summary string) ([]string, error) {
	system := `从以下工作摘要中提取明确的风险项。只有以下情况才算风险：
- 明确提到"阻塞"、"卡住"、"无法继续"
- 明确提到"延期"、"来不及"、"deadline 赶不上"
- 明确提到线上故障、生产环境问题仍未解决
- 明确提到需要其他人/团队支持但未获得

以下情况不算风险：
- 修复了 bug、解决了问题 — 这是正常工作成果
- 任务进行中、完成一部分 — 正常进展
- 计划明天做、下周做 — 正常排期

返回 JSON：{"risks":["风险描述"]}，无风险则空数组。只返回 JSON。`
	result, err := s.chat(ctx, system, summary)
	if err != nil {
		return nil, fmt.Errorf("detect risks: %w", err)
	}
	var parsed struct {
		Risks []string `json:"risks"`
	}
	if json.Unmarshal([]byte(result), &parsed) != nil {
		return nil, nil
	}
	return parsed.Risks, nil
}

// StreamQueryAnswer 通过 Data Asking 流式回答查询（带 session 上下文）
func (s *AIService) StreamQueryAnswer(ctx context.Context, question string, sessionID string, flush func(string), thinkFlush func(string)) error {
	if s.raw == nil || s.catalogDBID == 0 {
		flush("Data Asking 未配置，无法查询。")
		return nil
	}

	stream, err := s.raw.AnalyzeDataStream(ctx, &sdk.DataAnalysisRequest{
		Question:  question,
		SessionID: strPtr(sessionID),
		Config: &sdk.DataAnalysisConfig{
			DataSource: &sdk.DataSource{
				Type: "specified",
				Tables: &sdk.DataAskingTableConfig{
					Type:      "specified",
					DbName:    s.dbName,
					TableList: []string{"daily_entries", "members", "daily_summaries"},
				},
			},
			DataScope:     &sdk.DataScope{Type: "all"},
			ContextConfig: &sdk.ContextConfig{MaxKnowledgeItems: 20, MaxKnowledgeValueLength: 200},
		},
	})
	if err != nil {
		return fmt.Errorf("data asking: %w", err)
	}
	defer stream.Close()

	for {
		event, err := stream.ReadEvent()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read event: %w", err)
		}
		if event == nil {
			continue
		}

		switch {
		case event.StepType == "decomposition":
			thinkFlush("正在分析问题...")
		case event.StepType == "exploration":
			thinkFlush("正在探索数据表结构...")
		case event.StepType == "agent_reasoning":
			if msg, ok := event.Data["message"].(string); ok {
				runes := []rune(msg)
				if len(runes) > 80 {
					msg = string(runes[:80]) + "..."
				}
				thinkFlush(msg)
			}
		case event.StepType == "sql_generation":
			thinkFlush("正在生成查询语句...")
		case event.StepType == "sql_execution":
			thinkFlush("查询完成，正在整理结果...")
		case event.StepType == "insight":
			s.flushInsightBlocks(event.Data, flush)
		}
	}
	return nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *AIService) flushInsightBlocks(data map[string]interface{}, flush func(string)) {
	blocks, ok := data["blocks"].([]interface{})
	if !ok {
		return
	}
	for _, b := range blocks {
		block, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		if text, ok := block["text"].(map[string]interface{}); ok {
			if content, ok := text["content"].(string); ok {
				flush(content)
			}
		}
	}
}

// MergeDailySummary 将今天所有提交记录合并成一份总结
func (s *AIService) MergeDailySummary(ctx context.Context, entries []string) (string, error) {
	system := `你是日报合并助手。将用户多次提交的工作记录合并为一份简洁的当日总结。
规则：
- 如果后面的记录修正了前面的内容，以最新为准
- 去重，合并相同事项
- 每条以 • 开头，直接输出合并后的摘要`
	user := "以下是今天多次提交的工作记录（按时间顺序）：\n" + strings.Join(entries, "\n---\n")
	return s.doChatWithModel(ctx, s.fastModel, system, user, false, nil)
}

// StreamWeeklySummary 流式生成周报，返回完整内容用于保存文件
func (s *AIService) StreamWeeklySummary(ctx context.Context, userName, data string, flush func(string)) (string, error) {
	system := `根据日报数据生成 Markdown 周报，包含：# 周报 - {姓名}、## 本周重点、## 进展详情、## 风险与阻塞、## 下周计划。`
	prompt := fmt.Sprintf("姓名：%s\n日报数据：\n%s", userName, data)
	return s.stream(ctx, system, prompt, flush)
}

// DateRange 表示 LLM 从自然语言提取的日期范围
type DateRange struct {
	Start string `json:"start"` // YYYY-MM-DD
	End   string `json:"end"`   // YYYY-MM-DD
}

// ExtractDateRange 从用户输入中提取日期范围，默认最近7天
func (s *AIService) ExtractDateRange(ctx context.Context, text, today, weekday string) (*DateRange, error) {
	system := fmt.Sprintf(`你是日期解析助手。今天是 %s（%s）。
用户会用自然语言描述一个时间范围，请提取为精确日期。
规则：
- "本周"指本周一到今天
- "上周"指上周一到上周日
- "最近一周"指过去7天
- "前两周"指过去14天
- 如果用户没有明确时间，默认最近7天
只输出 JSON：{"start":"YYYY-MM-DD","end":"YYYY-MM-DD"}`, today, weekday)
	result, err := s.doChatWithModel(ctx, s.fastModel, system, text, false, nil)
	if err != nil {
		return nil, err
	}
	// 提取 JSON
	result = strings.TrimSpace(result)
	if i := strings.Index(result, "{"); i >= 0 {
		if j := strings.LastIndex(result, "}"); j > i {
			result = result[i : j+1]
		}
	}
	var dr DateRange
	if err := json.Unmarshal([]byte(result), &dr); err != nil {
		return nil, fmt.Errorf("parse date range: %w (raw: %s)", err, result)
	}
	return &dr, nil
}

// ClassifyIntent 用 qwen-turbo 快速判断意图（带对话历史上下文）
func (s *AIService) ClassifyIntent(ctx context.Context, text string, history []map[string]string) (string, error) {
	system := `判断用户输入的意图，结合对话历史理解上下文，返回一个词：
- report：用户在描述自己做了什么工作（如"今天修了个bug"、"完成了XX功能开发"）
- query：用户在查询数据、统计、查看日报、查人员信息（如"张三最近做了什么"、"本周团队进展"）
- chat：闲聊、问候、感谢、提问、求建议、补充说明等其他情况
注意：如果用户在补充之前对话的信息（如之前聊了工作内容，现在补充细节），应归为之前的意图类型。
只返回一个词。`
	result, err := s.doChatWithHistory(ctx, s.fastModel, system, history, text, false, nil)
	if err != nil {
		return "chat", err
	}
	r := strings.ToLower(strings.TrimSpace(result))
	for _, intent := range []string{"report", "query", "chat"} {
		if strings.Contains(r, intent) {
			return intent, nil
		}
	}
	return "chat", nil
}

// StreamChat 闲聊流式回复（带上下文）
func (s *AIService) StreamChat(ctx context.Context, text string, history []map[string]string, flush func(string)) error {
	system := `你是 MOI 智能日报助手。友好简洁地回复用户。
严格规则：
- 绝对不要编造任何工作内容、日报数据、进展或统计信息
- 不要假装"已记录"或"已保存"任何内容，你没有记录功能
- 不要生成日报、周报或任何报告内容
- 如果用户想提交日报，引导他们点击"汇报今日工作"按钮
- 如果用户想查数据，引导他们点击"查询团队动态"按钮
- 如果用户的消息不完整或含义不清，直接问清楚，不要猜测补全`
	_, err := s.doChatWithHistory(ctx, s.model, system, history, text, true, flush)
	return err
}

// StreamEmptyQueryFallback 查询无结果时，用思考过程上下文生成友好回复
func (s *AIService) StreamEmptyQueryFallback(ctx context.Context, question string, thinkingContext string, flush func(string)) error {
	system := `你是数据查询助手。用户提了一个数据查询问题，系统已经查询但没有找到结果。
根据以下思考过程的上下文，用自然友好的语言告诉用户查询结果（为空的原因），并给出建议。
不要编造数据，如实说明未查到。简洁回复，2-3句话即可。`
	user := fmt.Sprintf("用户问题：%s\n\n查询过程摘要：%s", question, thinkingContext)
	_, err := s.stream(ctx, system, user, flush)
	return err
}
