# CommitLog PRD：文章模板功能（从模板创建）

## 1. 背景与目标

当前后台写作流程已具备草稿自动保存、发布快照、AI 改写等能力，但新建文章仍需要重复输入固定结构（标题层级、固定段落、常用标签等），导致创作启动成本偏高。

本需求目标：

1. 支持预先创建文章模板。
2. 支持新建文章时从模板快速创建草稿。
3. 保障“模板与文章数据隔离”，模板后续修改不影响已创建文章。
4. 保持与现有草稿/发布流程兼容，不破坏当前编辑器与 API 行为。

## 2. 范围定义

### 2.1 In Scope（本期）

1. 模板管理：新增、编辑、删除、列表查询、详情预览。
2. 从模板创建文章：创建新草稿并跳转编辑页。
3. 模板字段复制：`content`、`summary`、`visibility`、`cover`、`tags`。
4. 来源追踪：文章记录 `source_template_id`。
5. 基础占位符：`{{date}}`、`{{datetime}}`、`{{title}}`。

### 2.2 Out of Scope（本期不做）

1. 模板版本历史与回滚。
2. 模板协作审批流。
3. 复杂逻辑占位符（条件分支、循环、脚本执行）。
4. 多租户团队权限模型（先兼容单管理员场景，结构预留）。

## 3. 用户故事

1. 作为内容创作者，我希望维护多个模板（周报、技术复盘、产品日志），减少重复输入。
2. 作为内容创作者，我希望点击“从模板创建”后直接得到可编辑草稿。
3. 作为内容创作者，我希望模板更新只影响未来新文章，不影响历史文章。
4. 作为管理者，我希望知道模板使用频次，判断是否保留或优化模板。

## 4. 产品方案

## 4.1 信息架构与入口

1. 后台新增页面：`/admin/post-templates`（模板管理）。
2. 文章列表页“新建文章”入口拆分为：
   - `空白新建`（现有 `/admin/posts/new`）
   - `从模板创建`（打开模板选择弹窗）
3. 编辑页显示来源信息（只读）：`来源模板：xxx`（若存在）。

### 4.2 流程说明

#### 4.2.1 创建模板

1. 在模板管理页点击“新建模板”。
2. 填写模板名称、内容、默认摘要、可见性、封面、标签。
3. 保存成功后出现在模板列表，可预览和编辑。

#### 4.2.2 从模板创建文章

1. 在文章管理页点击“从模板创建”。
2. 选择模板并确认（如果内容里有 `{{title}}` 占位符弹窗让用户输入做填充替换）。
3. 系统执行占位符替换并创建一篇新的 `draft` 文章。
4. 跳转到 `/admin/posts/{id}/edit`，进入现有编辑流程。

#### 4.2.3 隔离行为

1. 模板创建文章时使用内容快照复制（非引用）。
2. 之后编辑文章不回写模板。
3. 更新模板不影响历史文章。

## 5. 数据模型设计

> 设计原则：复用现有 `Post` 数据结构，尽量最小增量。

### 5.1 新增表：`post_templates`

建议字段（GORM）：

1. `id`（主键）
2. `created_at` / `updated_at`
3. `name` `varchar(128) not null`
4. `description` `varchar(512) default ''`
5. `content` `text not null`
6. `summary` `text default ''`
7. `visibility` `varchar(16) not null default 'public'`
8. `cover_url` `text default ''`
9. `cover_width` `int default 0`
10. `cover_height` `int default 0`
11. `usage_count` `int not null default 0`
12. `last_used_at` `datetime null`

索引建议：

1. `idx_post_templates_name (name)`

### 5.2 新增关联表：`post_template_tags`

1. `post_template_id`
2. `tag_id`

### 5.3 文章表扩展：`posts`

新增字段：

1. `source_template_id` `int null`

约束建议：

1. 外键到 `post_templates.id`，`ON DELETE SET NULL`。
2. 历史文章保留，不因模板删除而失效。

## 6. 占位符规则

## 6.1 支持变量

1. `{{date}}`：按 `YYYY-MM-DD`。
2. `{{datetime}}`：按 `YYYY-MM-DD HH:mm`。
3. `{{title}}`：从“本次标题输入”获取；若未输入则为空。

### 6.2 替换时机

1. 仅在“从模板创建文章”时执行一次替换。
2. 模板内容本身不被改写。

### 6.3 异常处理

1. 未知变量保留原文，不报错。
2. 替换失败返回 500，并带明确错误信息（遵循 fast fail）。

## 7. API 设计（后台）

统一前缀：`/admin/api`。

### 7.1 模板管理 API

1. `GET /post-templates`
   - 查询参数：`keyword`、`page`、`per_page`
   - 返回：模板列表 + 分页 + 统计
2. `GET /post-templates/:id`
   - 返回单模板详情
3. `POST /post-templates`
   - 入参：`name, description, content, summary, visibility, cover_*, tag_ids`
4. `PUT /post-templates/:id`
   - 入参同上
5. `DELETE /post-templates/:id`
   - 物理删除（若被文章引用，依赖 `ON DELETE SET NULL` 解除关联）

### 7.2 从模板创建文章 API

1. `POST /posts/from-template`
2. 入参：
   - `template_id`（必填）
   - `title`（可选，用于 `{{title}}`）
3. 出参：
   - `message`
   - `post`（与现有 `CreatePost` 返回结构一致）

### 7.3 服务层建议

1. 新增 `TemplateService`：模板 CRUD、变量替换、统计更新。
2. `PostService` 新增能力：`CreateFromTemplate(templateID, opts)`。
3. 创建流程需事务化：
   - 读取模板 + tags
   - 执行变量替换
   - 创建 post（draft）
   - 关联 tags
   - 更新模板 `usage_count/last_used_at`

## 8. 页面与交互设计

### 8.1 模板管理页 `/admin/post-templates`

1. 列表字段：名称、最近使用、使用次数、更新时间。
2. 操作：编辑、删除、预览、从该模板创建文章。
3. 顶部按钮：新建模板。

### 8.2 文章管理页 `/admin/posts`

1. “新建文章”按钮改为 split button。
2. 点击“从模板创建”弹出模板选择面板：
   - 搜索
   - 模板卡片预览（摘要 + 标签 + 更新时间）
   - 标题输入（可选）
   - 确认创建

### 8.3 编辑页 `/admin/posts/:id/edit`

1. 元信息区显示来源模板名称（若有）。
2. 可选提供“另存为模板”入口（P1，可后置）。

## 9. 权限与安全

1. 复用现有后台认证中间件 `AuthRequired`。
2. 本期默认当前用户可管理自己的模板（单管理员即全部可管理）。
3. 变量替换不执行表达式，不允许任意代码注入。

## 10. 兼容性与迁移

1. 通过 `AutoMigrate` 增量创建 `post_templates`、`post_template_tags`、`posts.source_template_id`。
2. 历史数据无需回填，`source_template_id` 为空即可。
3. 若模板删除，历史文章保留，仅来源字段置空（按外键策略）。

## 11. 测试方案

## 11.1 Go 单元测试（必做）

1. `internal/service/template_service_test.go`
   - 模板 CRUD
   - 占位符替换
   - 物理删除后列表与详情行为
2. `internal/service/post_service_test.go`
   - 从模板创建成功
   - 模板不存在失败
   - 模板与文章隔离验证
3. `internal/handler/post_template_handler_test.go`
   - API 参数校验、错误码与消息断言

### 11.2 集成与回归测试

1. 新增 `internal/router/router_test.go` 路由可达性断言。
2. 回归现有文章创建、编辑、发布、草稿恢复流程。

### 11.3 Playwright（Web 功能验证）

新增场景（`tests/`）：

1. `post_template_crud.test.js`
2. `post_create_from_template.test.js`
3. `post_template_isolation.test.js`

关键截图输出到：

1. `output/playwright/template-list.png`
2. `output/playwright/create-from-template-dialog.png`
3. `output/playwright/create-from-template-editor-filled.png`

## 12. 验收标准（Definition of Done）

1. 管理员可完成模板的新增、编辑、删除。
2. 可从模板创建新文章，正文/摘要/标签/可见性/封面按预期填充。
3. 新文章修改不影响模板；模板修改不影响历史文章。
4. 模板删除后，历史文章仍可正常展示和编辑。
5. 文章记录 `source_template_id`，并可用于统计。
6. `make build`、`make test`、`make lint` 全部通过。

## 13. 实施拆分建议（小步提交）

1. `feat(db):` 模板表与文章来源字段迁移。
2. `feat(service):` TemplateService + CreateFromTemplate。
3. `feat(api):` 模板管理与创建接口。
4. `feat(admin-ui):` 模板管理页 + 创建入口弹窗。
5. `test(e2e):` Playwright 场景与截图。

## 14. 风险与对策

1. 风险：模板正文过大导致创建卡顿。
   - 对策：限制模板内容长度并在前后端双重校验。
2. 风险：占位符替换引起不可预期文本。
   - 对策：替换白名单 + 未知变量保留原文。
3. 风险：模板删除影响历史关联。
   - 对策：`ON DELETE SET NULL`，文章侧仅保留 `source_template_id` 历史信息，不阻塞后续编辑与发布。

---

该方案默认与当前仓库实现保持一致：文章标题依旧由 Markdown 首行推导，不单独新增数据库 title 字段。
