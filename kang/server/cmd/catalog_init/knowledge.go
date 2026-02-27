package main

import (
	"context"
	"smart-daily/internal/logger"

	sdk "github.com/matrixorigin/moi-go-sdk"
)

func initKnowledge(ctx context.Context, client *sdk.RawClient) error {
	knowledges := []sdk.NL2SQLKnowledgeCreateRequest{
		// glossary: 术语解释
		{Type: "glossary", Key: "日报", Value: []string{"daily_entries表中的一条记录，代表某个团队成员某天提交的工作汇报"}},
		{Type: "glossary", Key: "成员", Value: []string{"members表中的记录，代表团队中的一个人"}},
		{Type: "glossary", Key: "风险", Value: []string{"daily_summaries.risk字段，AI从日报内容中检测到的潜在风险项"}},
		{Type: "glossary", Key: "摘要", Value: []string{"daily_summaries.summary字段，AI对日报原始内容的精炼总结"}},

		// synonyms: 同义词→表.列映射
		{Type: "synonyms", Key: "姓名/名字/谁/人员/同事", Value: []string{"团队成员的真实姓名"}, AssociateTables: []string{"members,name"}},
		{Type: "synonyms", Key: "日期/时间/哪天/什么时候", Value: []string{"日报的日期"}, AssociateTables: []string{"daily_entries,daily_date"}},
		{Type: "synonyms", Key: "工作内容/做了什么/干了啥", Value: []string{"日报原始内容"}, AssociateTables: []string{"daily_entries,content"}},
		{Type: "synonyms", Key: "角色/职位/岗位", Value: []string{"成员的职位角色"}, AssociateTables: []string{"members,role"}},

		// logic: 业务逻辑
		{Type: "logic", Key: "查询某人的日报时，需要通过daily_entries.member_id关联members.id来获取姓名", Value: []string{"JOIN members ON daily_entries.member_id = members.id"}},
		{Type: "logic", Key: "今天指CURDATE()，本周指从本周一到今天，本月指从本月1号到今天", Value: []string{"日期范围计算规则"}},
		{Type: "logic", Key: "判断谁没交日报：用members LEFT JOIN daily_entries，找daily_entries.id IS NULL的成员", Value: []string{"缺勤查询逻辑"}},
		{Type: "logic", Key: "统计提交率：已提交人数/总人数*100，排除role='测试'的测试账号", Value: []string{"提交率计算排除测试账号"}},

		// case_library: 问答样例
		{Type: "case_library", Key: "今天谁没交日报", Value: []string{"SELECT m.name FROM members m LEFT JOIN daily_entries de ON m.id = de.member_id AND de.daily_date = CURDATE() WHERE de.id IS NULL AND m.role != '测试'"}},
		{Type: "case_library", Key: "彭振这周做了什么", Value: []string{"SELECT de.daily_date, de.summary FROM daily_entries de JOIN members m ON de.member_id = m.id WHERE m.name = '彭振' AND de.daily_date >= DATE_SUB(CURDATE(), INTERVAL WEEKDAY(CURDATE()) DAY)"}},
		{Type: "case_library", Key: "本周有哪些风险", Value: []string{"SELECT m.name, ds.daily_date, ds.risk FROM daily_summaries ds JOIN members m ON ds.member_id = m.id WHERE ds.risk != '' AND ds.daily_date >= DATE_SUB(CURDATE(), INTERVAL WEEKDAY(CURDATE()) DAY)"}},
		{Type: "case_library", Key: "最近一周的日报提交情况", Value: []string{"SELECT de.daily_date, COUNT(*) as submitted FROM daily_entries de WHERE de.daily_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY) GROUP BY de.daily_date ORDER BY de.daily_date"}},
	}

	for _, k := range knowledges {
		resp, err := client.CreateKnowledge(ctx, &k)
		if err != nil {
			if isDuplicate(err) {
				logger.Info("knowledge: already exists, skipping", "type", k.Type, "key", k.Key)
				continue
			}
			return err
		}
		logger.Info("knowledge: created", "type", k.Type, "key", k.Key, "id", resp.ID)
	}
	return nil
}
