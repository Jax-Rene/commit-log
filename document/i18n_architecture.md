# 双语架构设计（中文 / English）

## 目标与原则
- **同一个域名**：使用路径前缀区分语言。 
- **双语覆盖**：所有页面均支持中文/英文。 
- **默认策略**：中文用户 → 中文，海外用户 → 英文。 
- **可维护**：清晰的职责边界与最小复杂度。 
- **SEO 友好**：规范的 `hreflang`、`canonical`、sitemap。 
- **专业度**：结构清晰、可扩展、实现路径明确。 

---

## 1. 路由与 URL 规则
### 1.1 URL 规范
- 中文：`/zh/...`
- 英文：`/en/...`

**示例**
- 首页：`/zh/`、`/en/`
- 文章页：`/zh/post/{slug}`、`/en/post/{slug}`
- 标签页：`/zh/tag/{slug}`、`/en/tag/{slug}`

### 1.2 路由约定
- 所有公开页面均以 `/{lang}` 作为第一层路径。
- `lang` 仅允许 `zh`、`en`。其它路径直接 404 或重定向到默认语言。 

### 1.3 统一路由中间件
- 解析 URL 第一段作为 `lang`。
- 将 `lang` 存入上下文（用于模板渲染、查询、SEO）。
- 对于缺失 `lang` 的请求，执行语言判定逻辑并重定向。 

---

## 2. 语言切换机制
### 2.1 右上角语言切换（UI）
**展示**：`中 / EN`

**切换规则**
- 当前 `/zh/...` → 切换到 `/en/...`
- 当前 `/en/...` → 切换到 `/zh/...`

**URL 跳转规则**
- 保持剩余路径不变，仅替换 `lang` 前缀。
- 示例：
  - `/zh/post/hello` → `/en/post/hello`
  - `/en/tag/golang` → `/zh/tag/golang`

**cookie / localStorage 使用方式**
- `cookie`：`blog_lang=zh|en`（服务端优先读取）
- `localStorage`：`blog_lang`（前端切换时更新，回写 cookie）
- 统一策略：写入两端，读优先级由服务端控制。

**内容缺失降级策略**
- 当目标语言内容不存在：
  1. 展示另一语言版本（fallback）。
  2. 页面明显提示「当前语言版本暂缺」并提供切回按钮。

### 2.2 首次访问自动语言判断
**判定优先级**
1. 用户历史选择（cookie/localStorage）
2. `Accept-Language` 头部
3. System Settings 的首选语言

**Accept-Language 判断规则**
- 包含 `zh`（`zh-CN/zh-Hans`等）→ `zh`
- 包含 `en` → `en`
- 无法匹配 → fallback 到系统默认语言

---

## 3. 内容数据模型设计
### 3.1 核心数据结构
- `posts`：统一内容骨架，存储与语言无关的信息（ID、作者、状态、发布时间等）。
- `post_translations`：存储不同语言版本的内容。 
- `tags`：语言无关的标签实体，包含 `zh/en` 名称与描述（slug 统一、用于关联）。

**示意结构**
```
posts
- id
- status
- published_at
- ...

post_translations
- id
- post_id
- lang (zh/en)
- slug
- title
- summary
- content
- seo_title
- seo_description
- ...

tags
- id
- slug
- name_zh
- name_en
- description_zh
- description_en
```

### 3.2 查询逻辑
- 根据 `lang` 优先加载对应翻译。
- 若无对应翻译 → fallback 到另一种语言（并标记 fallback）。
- 列表页也需做语言过滤与 fallback。

---

## 4. 内容生成与维护策略
### 4.1 创建文章
- 后台新增文章提供「语言」下拉选项（默认选当前页面语言）。
- 创建后进入对应语言版本编辑页。

### 4.2 翻译辅助（AI）
- 内容管理页增加「一键翻译」按钮。
- 以当前语言内容为输入，生成对等翻译写入 `post_translations`。
- 翻译结果需要人工二次编辑确认。 

### 4.3 slug 生成规则（slug 使用英文内容）
- **目标**：中文与英文内容各自独立生成 slug，但 slug 均为英文表达（允许不同）。
- **优先策略（有 AI 服务时）**：
  1. 英文版本：normalized title 作为 slug（例如：how-i-designed-a-bilingual-blog）
  2. 中文版本：使用 AI 根据语义生成英文 slug（例如：how-to-write-blog）
- **兜底策略（无 AI 时）**：
  1. 对原始标题做规范化（trim、大小写、标点统一）。
  2. 中文标题使用拼音生成英文 slug（统一为小写 + `-` 分隔）
- **注意**：slug 在对应语言版本创建时固定，后续翻译版本各自生成，不互相复用。

### 4.4 标签双语维护
- 标签编辑页支持配置 `zh/en` 名称与描述。
- 前台按语言展示对应字段，缺失时 fallback 另一种语言。

---

## 5. SEO 与专业度设计
### 5.1 hreflang
在 `<head>` 中添加：
```
<link rel="alternate" hreflang="zh" href="https://example.com/zh/post/hello" />
<link rel="alternate" hreflang="en" href="https://example.com/en/post/hello" />
<link rel="alternate" hreflang="x-default" href="https://example.com/zh/post/hello" />
```

### 5.2 canonical
- 每个语言版本都有自己的 canonical。
- 指向当前语言版本 URL，避免内容重复的 SEO 风险。

### 5.3 sitemap
- Sitemap 中区分语言 URL。
- 每条内容输出两个语言版本（若存在）。

---

## 6. 内容缺失时的用户体验
- 页面顶部显示提示：
  - 「该内容暂无中文版本，已展示英文版本。」
  - 「This content is not available in English, showing Chinese version.」
- 提供快速切回入口。

---

## 7. 系统配置（System Settings）
- 新增配置项：`default_language`（`zh` / `en`）。
- 用于当用户无历史选择、Accept-Language 无法判断时的默认语言。 

## 8. AI 总结
- 不同语言文章独立做 Summary 内容独立存储

---

## 9. 工程落地建议（分阶段）
### Phase 1：路由与语言选择
- 加入 `/{lang}` 路由前缀。
- 语言中间件 + cookie/localStorage 机制。
- 基础模板支持语言切换 UI。

### Phase 2：数据层扩展
- 新增 `post_translations` 表。
- 列表/详情查询支持多语言。
- 缺失语言 fallback 逻辑。

### Phase 3：后台管理与 SEO
- 管理后台增加语言编辑入口。
- AI 翻译支持（可配置）。
- 输出 `hreflang`、`canonical`、多语言 sitemap。

### Phase 4：数据迁移脚本
- 为存量数据提供迁移脚本（例如 `scripts/migrate_i18n_data.go`）。 
- 迁移规则：
  1. 旧文章写入 `posts`，原内容写入 `post_translations`（默认 `zh` 或系统配置）。
  2. 旧标签写入 `tags`，名称写入 `name_zh`（或根据系统默认语言写入对应字段）。
  3. 自动生成 slug，遵循统一 slug 规则。
  4. 迁移过程可重复执行（幂等）。
  5. 做好备份和恢复的手段。
  6. 迁移完成后支持手动删除备份数据。
