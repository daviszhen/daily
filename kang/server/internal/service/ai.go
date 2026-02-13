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
	baseURL string
	apiKey  string
	client  *http.Client
	raw     *sdk.RawClient
}

func NewAIService(baseURL, apiKey string, raw *sdk.RawClient) *AIService {
	return &AIService{baseURL: baseURL, apiKey: apiKey, client: &http.Client{}, raw: raw}
}

func (s *AIService) doChat(ctx context.Context, system, user string, stream bool, flush func(string)) (string, error) {
	body := map[string]interface{}{
		"model":  "qwen-plus",
		"stream": stream,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
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
	system := `判断用户输入是否为工作内容（如完成任务、进展、问题等）。
返回 JSON：{"valid":true} 或 {"valid":false,"reply":"友好引导语"}。只返回 JSON。`

	result, err := s.chat(ctx, system, content)
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

// StreamSummarize 流式生成工作摘要（纯文本，每条 • 开头）
func (s *AIService) StreamSummarize(ctx context.Context, content string, flush func(string)) (string, error) {
	system := `你是日报摘要助手。用简洁要点总结用户的工作内容，每条以 • 开头，直接输出摘要文本。`
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
- 明确提到"bug"、"故障"、"线上问题"等已发生的技术问题
- 明确提到需要其他人/团队支持但未获得

以下情况不算风险：
- 任务进行中、完成一部分（如"完成50%"）— 这是正常进展
- 计划明天做、下周做 — 这是正常排期
- 一般性的待办事项

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

// StreamQueryAnswer 通过 Data Asking 流式回答查询
func (s *AIService) StreamQueryAnswer(ctx context.Context, question string, flush func(string), thinkFlush func(string)) error {
	if s.raw == nil {
		flush("Data Asking 未配置，无法查询。")
		return nil
	}

	dbID := 12558898
	stream, err := s.raw.AnalyzeDataStream(ctx, &sdk.DataAnalysisRequest{
		Question: question,
		Config: &sdk.DataAnalysisConfig{
			DataSource: &sdk.DataSource{
				Type: "specified",
				Tables: &sdk.DataAskingTableConfig{
					Type: "all", DbName: "smart_daily", DatabaseID: &dbID,
				},
			},
			DataScope: &sdk.DataScope{Type: "all"},
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

// StreamWeeklySummary 流式生成周报，返回完整内容用于保存文件
func (s *AIService) StreamWeeklySummary(ctx context.Context, userName, data string, flush func(string)) (string, error) {
	system := `根据日报数据生成 Markdown 周报，包含：# 周报 - {姓名}、## 本周重点、## 进展详情、## 风险与阻塞、## 下周计划。`
	prompt := fmt.Sprintf("姓名：%s\n日报数据：\n%s", userName, data)
	return s.stream(ctx, system, prompt, flush)
}
