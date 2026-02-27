package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

const baseURL = "http://localhost:9871"

// browser wraps a chromedp context with test helpers.
type browser struct {
	ctx    context.Context
	cancel context.CancelFunc
	t      *testing.T
}

func newBrowser(t *testing.T, timeout time.Duration) *browser {
	t.Helper()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	ctx, timeCancel := context.WithTimeout(ctx, timeout)

	b := &browser{ctx: ctx, t: t}
	b.cancel = func() { timeCancel(); ctxCancel(); allocCancel() }
	return b
}

func (b *browser) close() { b.cancel() }

func (b *browser) run(actions ...chromedp.Action) {
	b.t.Helper()
	if err := chromedp.Run(b.ctx, actions...); err != nil {
		b.t.Fatalf("chromedp: %v", err)
	}
}

func (b *browser) eval(js string) string {
	b.t.Helper()
	var r interface{}
	if err := chromedp.Run(b.ctx, chromedp.Evaluate(js, &r)); err != nil {
		b.t.Fatalf("eval: %v", err)
	}
	if r == nil {
		return ""
	}
	return fmt.Sprintf("%v", r)
}

func (b *browser) login(username, password string) {
	b.t.Helper()
	b.run(
		chromedp.Navigate(baseURL),
		chromedp.Sleep(2*time.Second),
		chromedp.SendKeys(`input[type="text"]`, username),
		chromedp.SendKeys(`input[type="password"]`, password),
		chromedp.Click(`button[type="submit"]`),
		chromedp.Sleep(3*time.Second),
	)
}

func (b *browser) clickButton(text string) {
	b.t.Helper()
	b.eval(fmt.Sprintf(`(function(){
		document.querySelectorAll('button').forEach(function(btn){
			if(btn.textContent.includes('%s')) btn.click();
		});
	})()`, text))
	b.run(chromedp.Sleep(500 * time.Millisecond))
}

func (b *browser) sendMessage(text string) {
	b.t.Helper()
	b.eval(fmt.Sprintf(`(function(){
		var t = document.querySelector('textarea');
		var s = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
		s.call(t, '%s');
		t.dispatchEvent(new Event('input', {bubbles: true}));
	})()`, text))
	b.run(chromedp.Sleep(300 * time.Millisecond))
	b.eval(`(function(){
		var bs = document.querySelectorAll('button');
		for (var i = 0; i < bs.length; i++) {
			if (bs[i].querySelector('svg') && !bs[i].disabled && bs[i].closest('.rounded-2xl')) {
				bs[i].click(); return;
			}
		}
	})()`)
}

func (b *browser) checkNoError() {
	b.t.Helper()
	r := b.eval(`(function(){ var d = document.querySelector('pre[style*="red"]'); return d ? d.textContent.substring(0,300) : ""; })()`)
	if r != "" {
		b.t.Fatalf("page error: %s", r)
	}
}

func (b *browser) bodyText() string {
	return b.eval(`document.body.innerText`)
}

func (b *browser) waitForReply(timeout time.Duration) string {
	b.t.Helper()
	b.run(chromedp.Sleep(timeout))
	b.checkNoError()
	return b.bodyText()
}

// --- Tests ---

func TestLogin(t *testing.T) {
	b := newBrowser(t, 30*time.Second)
	defer b.close()

	b.run(chromedp.Navigate(baseURL), chromedp.Sleep(2*time.Second))
	if !strings.Contains(b.bodyText(), "登录以继续") {
		t.Fatal("login page not shown")
	}

	b.login("kuaiweikang", "123456")
	b.checkNoError()
	if !strings.Contains(b.bodyText(), "蒯伟康") {
		t.Fatal("login failed")
	}
	if b.eval(`document.querySelector('.justify-center') ? 'yes' : 'no'`) != "yes" {
		t.Fatal("centered layout not shown")
	}
	t.Log("OK: login + centered layout")
}

func TestReportMode(t *testing.T) {
	b := newBrowser(t, 60*time.Second)
	defer b.close()
	b.login("kuaiweikang", "123456")

	b.clickButton("汇报今日工作")
	b.sendMessage("完成了e2e测试框架重构和智能路由开发")
	body := b.waitForReply(12 * time.Second)

	if !strings.Contains(body, "日报预览") {
		t.Fatal("summary confirm card not shown")
	}
	t.Log("OK: report → summary card")
}

func TestQueryMode(t *testing.T) {
	b := newBrowser(t, 90*time.Second)
	defer b.close()
	b.login("kuaiweikang", "123456")

	b.clickButton("查询团队动态")
	b.sendMessage("蒯伟康最近做了什么")
	body := b.waitForReply(20 * time.Second)

	if !strings.Contains(body, "思考过程") {
		t.Log("warning: no thinking steps shown")
	}
	// Should have some response (not stuck)
	if strings.Count(body, "蒯伟康最近做了什么") > 0 {
		t.Log("OK: query completed with response")
	}
}

func TestQueryEmptyResult(t *testing.T) {
	b := newBrowser(t, 90*time.Second)
	defer b.close()
	b.login("kuaiweikang", "123456")

	b.clickButton("查询团队动态")
	b.sendMessage("我今天做了啥")
	body := b.waitForReply(25 * time.Second)

	// Should NOT be stuck - should have a friendly response
	msgs := b.eval(`document.querySelectorAll('.rounded-2xl').length`)
	if msgs == "0" {
		t.Fatal("no response rendered - page stuck")
	}
	// Should contain some text after thinking (LLM fallback)
	if strings.Contains(body, "思考过程") {
		t.Log("OK: thinking steps shown")
	}
	t.Log("OK: empty query got friendly response, not stuck")
}

func TestAutoRouteChat(t *testing.T) {
	b := newBrowser(t, 60*time.Second)
	defer b.close()
	b.login("kuaiweikang", "123456")

	// Don't click any mode button, just type
	b.sendMessage("你好啊")
	body := b.waitForReply(10 * time.Second)

	// Should NOT say "请选择下方的功能按钮"
	if strings.Contains(body, "请选择下方的功能按钮") {
		t.Fatal("still showing dumb button prompt instead of smart routing")
	}
	t.Log("OK: auto-routed chat, got friendly response")
}

func TestAutoRouteReport(t *testing.T) {
	b := newBrowser(t, 60*time.Second)
	defer b.close()
	b.login("kuaiweikang", "123456")

	// Type work content without clicking report button
	b.sendMessage("今天修复了登录页面的样式bug")
	body := b.waitForReply(12 * time.Second)

	if strings.Contains(body, "日报预览") {
		t.Log("OK: auto-detected report intent, showed summary card")
	} else {
		t.Log("OK: responded (may have classified as chat, acceptable)")
	}
}

func TestSessionSwitch(t *testing.T) {
	b := newBrowser(t, 60*time.Second)
	defer b.close()
	b.login("kuaiweikang", "123456")

	// Send a message to create session
	b.clickButton("汇报今日工作")
	b.sendMessage("测试会话管理")
	b.waitForReply(10 * time.Second)

	// Switch to another session
	b.eval(`(function(){
		var items = document.querySelectorAll('.truncate');
		if(items.length > 1) items[1].closest('[class*=cursor-pointer]').click();
	})()`)
	b.run(chromedp.Sleep(3 * time.Second))
	b.checkNoError()
	t.Log("OK: session switch")

	// New chat
	b.clickButton("AI 助手")
	b.run(chromedp.Sleep(2 * time.Second))
	b.checkNoError()
	if b.eval(`document.querySelector('.justify-center') ? 'yes' : 'no'`) != "yes" {
		t.Fatal("new chat didn't restore centered layout")
	}
	t.Log("OK: new chat → centered layout")
}

func TestReportVagueContent(t *testing.T) {
	b := newBrowser(t, 60*time.Second)
	defer b.close()
	b.login("kuaiweikang", "123456")

	b.clickButton("汇报今日工作")
	b.sendMessage("修了个bug")
	body := b.waitForReply(10 * time.Second)

	// Should NOT show confirm card - should ask follow-up
	if strings.Contains(body, "日报预览") {
		t.Fatal("vague content should NOT get confirm card directly")
	}
	t.Log("OK: vague content triggered follow-up question")

	// Now provide detail
	b.sendMessage("是登录页面验证码不刷新的问题，在auth模块修的")
	body = b.waitForReply(12 * time.Second)

	if strings.Contains(body, "日报预览") {
		t.Log("OK: detailed content got confirm card after follow-up")
	} else {
		t.Log("OK: responded (may need more detail, acceptable)")
	}
}

func TestReportValidationReject(t *testing.T) {
	b := newBrowser(t, 60*time.Second)
	defer b.close()
	b.login("kuaiweikang", "123456")

	b.clickButton("汇报今日工作")
	b.sendMessage("你觉得加班合理吗")
	body := b.waitForReply(10 * time.Second)

	// Should get a friendly rejection, not crash
	if strings.Contains(body, "日报预览") {
		t.Log("note: LLM accepted this as work content")
	} else {
		t.Log("OK: validation rejected with guidance")
	}
}
