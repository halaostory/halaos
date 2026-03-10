# Implementation Plan: AI HR Enhancement — Employee & Employer 端 AI 集成

## Task Type
- [x] Frontend (Mobile Vant 4 + Desktop NaiveUI)
- [x] Backend (Go/Gin)
- [x] Fullstack (Parallel)

---

## Executive Summary

现有系统已具备 50+ AI tools、Agent 系统、SSE Streaming、Token 计费、知识库 RAG。方案中 70% 功能是"组合式"改进（更好的提示词 + 上下文 + 移动端 UI），30% 是真正新增基础设施。

**真正需要新建的核心**：
1. Server-side Draft→Confirm 确认机制（当前仅靠 LLM 指令，不可靠）
2. Context Injection 上下文注入框架（角色/页面/数据快照）
3. Mobile AI 全套组件（当前移动端零 AI 集成）
4. BYOK Key 管理层

**可直接复用无需大改**：
- 请假助手、薪资查询、政策问答、HR 分析 → 已有 tools + 更好的 prompts
- Agent 系统 → 已有 8 个预置 agent

---

## Technical Solution

### Architecture Decisions

| 决策 | 方案 | 理由 |
|------|------|------|
| Draft→Confirm | Hybrid: PostgreSQL (audit) + Redis (TTL cache) | 合规审计需要持久化，对话流需要低延迟 |
| Mobile AI 入口 | FloatingBubble + 内联卡片 | 不占 tab 位，跨页可达，5-tab 是移动端最佳实践 |
| Mobile Chat 布局 | 全屏路由页面（非 Popup） | iOS 键盘处理、返回手势、内容空间 |
| 代码共享 | shared/ 目录 + TS path alias | 单一事实来源，维护成本低 |
| OCR | Buy first (Google Vision / AWS Textract) | 自建成本高，MVP 先集成外部服务 |
| Context 注入 | Token 预算分级（1500 token 上限） | 避免 prompt 膨胀影响推理质量 |

---

## Implementation Steps

### Phase 1: Backend 安全基础设施（4 files）

#### Step 1.1: Action Draft 数据库迁移
新增 action_drafts + action_executions 表，含 status enum、risk_level、expires_at、idempotency_key。

#### Step 1.2: Draft Service (Go)
- internal/ai/draft/service.go — CreateDraft, ConfirmDraft, ExpireDrafts, GetDraft
- internal/ai/draft/handler.go — POST /v1/ai/drafts/confirm, GET /v1/ai/drafts/:id
- 核心：Tool executor 检测写操作→CreateDraft() 而非直接执行→SSE confirmation chunk→用户确认→执行

#### Step 1.3: Context Builder Service (Go)
- internal/ai/context/builder.go — BuildContext(companyID, userID, pageContext)
- 五层 token 预算: Identity(150) + Page(200) + Snapshot(400) + RAG(600) + Memory(150) = ~1500
- Redis 缓存 ctx:{company}:{user}:{page} TTL 60s

#### Step 1.4: 修改 Agent Executor 集成 Draft + Context
- executor.go 接收 pageContext 参数
- 写操作工具返回 DraftResult 而非直接执行
- SSE 新增 confirmation chunk type

### Phase 2: Shared 代码层（5 files）

#### Step 2.1: shared/ai/ 目录
- types.ts — ChatMessage, Agent, Session, DraftConfirmation, OcrResult
- api.ts — createAiAPI() 工厂函数（SSE streaming, agents, sessions, drafts）
- stream-parser.ts — SSE chunk 解析
- markdown.ts — markdown-it 配置
- i18n: ai-en.ts, ai-zh.ts

#### Step 2.2: 两个前端 TS path alias 配置
tsconfig.app.json + vite.config.ts 添加 @aigonhr/shared/* 映射

#### Step 2.3: Mobile API Client 集成 AI endpoints
client.ts 底部 import createAiAPI 并导出 aiAPI

### Phase 3: Mobile AI 核心组件（12 files）

#### Step 3.1: AiFloatingBubble.vue
- Vant FloatingBubble, bottom: 70px 避开 Tabbar
- 点击 router.push ai-chat, 未读 badge

#### Step 3.2: AiChatView.vue 全屏聊天页面
- height: 100dvh, flex column (NavBar + Messages + Input)
- SSE streaming 100ms rAF 节流
- Agent ActionSheet, Session Popup, 30s watchdog

#### Step 3.3: AI 子组件 (8个)
- AiChatMessages.vue — 消息列表 + auto-scroll
- AiChatInput.vue — safe-area-inset-bottom
- AiMessageBubble.vue — 用户/助手/系统消息
- AiConfirmCard.vue — 写操作确认卡片 (fields + Confirm/Cancel)
- AiSuggestionChips.vue — 横向滚动快捷建议
- AiAgentPicker.vue — Agent 选择
- AiSessionList.vue — 历史会话
- AiMarkdownRender.vue — Markdown 渲染

#### Step 3.4: Composables
- useAiChat.ts — SSE streaming, messages, session CRUD
- useAiContext.ts — 路由→page context

#### Step 3.5: Router + Layout 更新
- router 添加 ai-chat 路由
- MobileLayout 添加 AiFloatingBubble (v-if route != ai-chat)

### Phase 4: Mobile "AI Help Fill" + 内联增强（3 files）

#### Step 4.1: AiFormAssist.vue
- 调用 formPrefillAPI.get() → NoticeBar 建议 → 点击填充表单
- 失败静默降级

#### Step 4.2: LeaveView.vue 集成
- Apply tab 上方添加 AiFormAssist
- Reason 字段 AI 图标→ActionSheet 模板

#### Step 4.3: HomeView.vue AI 入口
- 快捷操作添加"AI 助手"Cell
- 打卡状态上下文建议

### Phase 5: Desktop ChatPanel 增强（3 files）

#### Step 5.1: ChatPanel Context Awareness
- 集成 useAiContext(), sendMessage 传 page_context
- 根据路由显示不同 suggestion chips

#### Step 5.2: AiSuggestionChips.vue (Desktop)
- NTag 列表, 按页面动态生成

#### Step 5.3: Batch Approval 增强
- Draft 检测→展开式列表→逐项 approve/reject
- Hard cap 25 项

### Phase 6: BYOK Key Management（2 files）

#### Step 6.1: 数据库迁移
api_keys_byok 表, AES-256-GCM 加密, key_hint

#### Step 6.2: Key Resolver Service
- Resolve(companyID, userID, provider) → apiKey
- 优先级: Company BYOK → User BYOK → Platform default
- Circuit breaker on failures

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| db/migrations/XXXXXX_action_drafts.up.sql | Create | Draft→Confirm 数据库表 |
| internal/ai/draft/service.go | Create | Draft 生命周期管理 |
| internal/ai/draft/handler.go | Create | Draft 确认 API |
| internal/ai/context/builder.go | Create | Context 分级注入 |
| internal/ai/agent/executor.go | Modify | 集成 draft + context |
| shared/ai/types.ts | Create | 共享 TS 类型 |
| shared/ai/api.ts | Create | AI API 工厂函数 |
| shared/ai/stream-parser.ts | Create | SSE 解析工具 |
| frontend-mobile/src/views/AiChatView.vue | Create | 移动端全屏聊天 |
| frontend-mobile/src/components/ai/*.vue | Create | 8 个 AI 组件 |
| frontend-mobile/src/composables/useAiChat.ts | Create | Chat composable |
| frontend-mobile/src/composables/useAiContext.ts | Create | Context composable |
| frontend-mobile/src/components/MobileLayout.vue | Modify | 添加 FloatingBubble |
| frontend-mobile/src/router/index.ts | Modify | 添加 ai-chat 路由 |
| frontend-mobile/src/api/client.ts | Modify | 添加 AI API |
| frontend/src/components/ChatPanel.vue | Modify | Context + suggestions |
| frontend/src/components/ai/AiSuggestionChips.vue | Create | Desktop suggestions |

---

## Risks and Mitigation

| Risk | Level | Mitigation |
|------|-------|------------|
| LLM 误解意图执行批量操作 | High | Draft→Confirm 强制两步走, batch cap 25 |
| Prompt 膨胀导致推理质量下降 | Medium | Token 预算分级, 每层独立上限 |
| 移动端键盘遮挡聊天输入 | Medium | 100dvh + safe-area-inset-bottom |
| BYOK key 泄露 | High | AES-256 加密, never log, circuit breaker |
| SSE 移动端弱网断连 | Medium | 30s watchdog + 重试按钮 |
| 敏感薪资数据残留在聊天 | High | 存储时 redact, 仅当次响应显示 |

---

## MVP Prioritization

| Priority | 内容 | 文件数 | 依赖 |
|----------|------|--------|------|
| P1 | Backend Draft→Confirm + Context Builder | 6 | None |
| P2 | Shared AI 代码层 | 5 | None |
| P3 | Mobile AI Chat 核心 | 12 | P1, P2 |
| P4 | Mobile AI FormAssist | 3 | P2 |
| P5 | Desktop ChatPanel Context | 3 | P1, P2 |
| P6 | BYOK Key Management | 2 | None |

建议 MVP 只做 P1-P3（~23 files）。

---

## SESSION_ID
- CODEX_SESSION: 019cd2bc-6fb7-7242-b5d4-e06cf59c9bcb
- GEMINI_SESSION: (failed)
