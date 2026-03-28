package bot

import (
	"context"
	"fmt"

	"github.com/tonypk/aigonhr/internal/store"
)

var breakTypeLabel = map[string]string{
	"meal": "吃饭", "bathroom": "上厕所", "rest": "休息", "leave_post": "中途离岗",
}

func (d *Dispatcher) handleBreakStart(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	// Check if already on break
	activeBreak, err := d.queries.GetActiveBreak(ctx, store.GetActiveBreakParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err == nil && activeBreak.ID > 0 {
		label := breakTypeLabel[activeBreak.BreakType]
		sender.SendWithKeyboard(ctx, msg.ChatID,
			fmt.Sprintf("⏳ 你正在休息中: %s\n开始时间: %s", label, activeBreak.StartAt.Format("15:04")),
			[][]InlineButton{
				{{Text: "结束休息", CallbackData: "bk:end"}},
			},
		)
		return
	}

	// Check if clocked in
	_, err = d.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err != nil {
		sender.SendText(ctx, msg.ChatID, "❌ 请先打卡上班")
		return
	}

	// Show break type selection keyboard
	sender.SendWithKeyboard(ctx, msg.ChatID, "选择休息类型:", [][]InlineButton{
		{
			{Text: "🍽 吃饭", CallbackData: "bk:meal"},
			{Text: "🚻 上厕所", CallbackData: "bk:bathroom"},
		},
		{
			{Text: "😌 休息", CallbackData: "bk:rest"},
			{Text: "🚪 中途离岗", CallbackData: "bk:leave_post"},
		},
	})
}

func (d *Dispatcher) handleBreakEnd(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	d.endActiveBreak(ctx, identity, msg.ChatID, sender)
}

func (d *Dispatcher) handleBreakStatus(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	activeBreak, err := d.queries.GetActiveBreak(ctx, store.GetActiveBreakParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err != nil {
		sender.SendText(ctx, msg.ChatID, "✅ 没有进行中的休息")
		return
	}

	label := breakTypeLabel[activeBreak.BreakType]
	sender.SendText(ctx, msg.ChatID,
		fmt.Sprintf("⏳ 当前休息: %s\n开始时间: %s", label, activeBreak.StartAt.Format("15:04")))
}

func (d *Dispatcher) handleBreakTypeCallback(ctx context.Context, cb CallbackQuery, identity *UserIdentity, breakType string, sender MessageSender) {
	// Verify clocked in
	att, err := d.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err != nil {
		sender.AnswerCallback(ctx, cb.ID, "请先打卡上班")
		return
	}

	// Check no active break
	existingBreak, err := d.queries.GetActiveBreak(ctx, store.GetActiveBreakParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err == nil && existingBreak.ID > 0 {
		sender.AnswerCallback(ctx, cb.ID, "已有进行中的休息")
		return
	}

	// Create break log
	breakLog, err := d.queries.CreateBreakLog(ctx, store.CreateBreakLogParams{
		CompanyID:       identity.CompanyID,
		EmployeeID:      identity.EmployeeID,
		AttendanceLogID: att.ID,
		BreakType:       breakType,
	})
	if err != nil {
		d.logger.Error("bot: failed to create break", "error", err)
		sender.AnswerCallback(ctx, cb.ID, "创建休息记录失败")
		return
	}

	label := breakTypeLabel[breakType]
	sender.AnswerCallback(ctx, cb.ID, "✅ 开始休息")
	sender.EditMessage(ctx, cb.ChatID, cb.MessageID,
		fmt.Sprintf("✅ 开始休息: %s\n⏰ 开始时间: %s", label, breakLog.StartAt.Format("15:04")))

	// Send end button
	sender.SendWithKeyboard(ctx, cb.ChatID,
		"休息进行中，完成后点击结束:",
		[][]InlineButton{
			{{Text: "结束休息", CallbackData: "bk:end"}},
		},
	)
}

func (d *Dispatcher) handleBreakEndCallback(ctx context.Context, cb CallbackQuery, identity *UserIdentity, sender MessageSender) {
	d.endActiveBreak(ctx, identity, cb.ChatID, sender)
	sender.AnswerCallback(ctx, cb.ID, "休息已结束")
}

func (d *Dispatcher) endActiveBreak(ctx context.Context, identity *UserIdentity, chatID string, sender MessageSender) {
	activeBreak, err := d.queries.GetActiveBreak(ctx, store.GetActiveBreakParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err != nil {
		sender.SendText(ctx, chatID, "❌ 没有进行中的休息")
		return
	}

	// Get policy for overtime calc
	var maxMinutes int32
	policy, err := d.queries.GetBreakPolicy(ctx, store.GetBreakPolicyParams{
		CompanyID: identity.CompanyID,
		BreakType: activeBreak.BreakType,
	})
	if err == nil {
		maxMinutes = policy.MaxMinutes
	}

	ended, err := d.queries.EndBreakLog(ctx, store.EndBreakLogParams{
		ID:      activeBreak.ID,
		Column2: maxMinutes,
	})
	if err != nil {
		d.logger.Error("bot: failed to end break", "error", err)
		sender.SendText(ctx, chatID, "❌ 结束休息失败")
		return
	}

	label := breakTypeLabel[ended.BreakType]

	var durVal int32
	if ended.DurationMinutes != nil {
		durVal = *ended.DurationMinutes
	}

	var otVal int32
	if ended.OvertimeMinutes != nil {
		otVal = *ended.OvertimeMinutes
	}

	if otVal > 0 {
		sender.SendText(ctx, chatID,
			fmt.Sprintf("⚠️ 休息结束: %s\n⏱ 时长: %d 分钟（超时 %d 分钟）", label, durVal, otVal))
	} else {
		sender.SendText(ctx, chatID,
			fmt.Sprintf("✅ 休息结束: %s\n⏱ 时长: %d 分钟", label, durVal))
	}
}
