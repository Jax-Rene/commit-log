# CommitLog 项目开发计划 Init

本文档基于 PRD 和技术选型文档，为项目 MVP 的实现提供一个分阶段、可执行的详细开发路线图。

---

## Phase 1: 地基搭建 (Backend Core & Project Setup)

**目标:** 初始化项目，搭建后端核心服务与数据库，并建立前端资源构建流程。

1.  **Go 项目初始化:**
    *   [x] 执行 `go mod init github.com/commitlog` 初始化 Go 模块。
    *   [x] 创建核心目录结构:
        *   `cmd/server`: Gin 服务启动入口。
        *   `internal/db`: 数据库连接、迁移和模型 (GORM)。
        *   `internal/handler`: Gin 的 HTTP handlers。
        *   `internal/service`: 核心业务逻辑。
        *   `internal/router`: 路由定义。
        *   `web/template`: HTML 模板。
        *   `web/static`: 存放编译后的 CSS/JS。

2.  **前端工具链搭建:**
    *   [x] 在项目根目录执行 `npm init -y`。
    *   [x] 安装前端依赖: `npm install -D tailwindcss` 并安装运行时依赖 `alpinejs`、`htmx.org`、`easymde` 等。
    *   [x] 创建 `tailwind.config.js` 并配置 `content` 路径，使其扫描 `web/template/**/*.html`。
    *   [x] 创建 `web/static/css/input.css` 并引入 Tailwind 指令。
    *   [x] 在 `package.json` 的 `scripts` 中添加 `build` 命令，使用 Vite 编译前端资源并输出到 `web/static/dist`。

3.  **数据库与模型 (GORM + SQLite):**
    *   [x] 在 `internal/db` 中引入 GORM 和 SQLite 驱动。
    *   [x] 定义数据模型 (`user.go`, `post.go`, `tag.go`)，包含 `gorm.Model`。
    *   [x] 实现数据库初始化函数，并使用 `AutoMigrate` 自动迁移建表。

4.  **核心 API 与路由 (Gin):**
    *   [x] 在 `internal/router` 中设置 Gin 引擎，并定义基础路由结构。
    *   [x] 在 `internal/handler` 中实现用户认证相关的 Handler (登录/注册逻辑框架)。
    *   [x] 实现简单的用户认证中间件。
    *   [x] 为文章 (Post) 和简历页 (About Me) 定义后端的 CRUD API 接口框架。

---

## Phase 2: 作者的驾驶舱 (Admin Panel UI)

**目标:** 构建后台管理界面，完成文章、简历的核心内容管理功能。

1.  **HTML 模板引擎集成:**
    *   [x] 在 Gin 中配置 `html/template` 引擎，加载 `web/template` 目录。
    *   [x] 创建后台主布局 `web/template/admin/layout.html`，引入通过 Vite 构建的静态资源并初始化 Alpine.js/HTMX。

2.  **后台核心页面开发 (HTML + Tailwind CSS):**
    *   [x] `login.html`: 登录表单页面。
    *   [x] `dashboard.html`: 后台主面板。
    *   [x] `post_list.html`: 文章管理列表。
    *   [x] `post_edit.html`: 文章创建/编辑页面，包含一个 `<textarea>` 用于 Markdown 编辑。

3.  **Markdown 编辑器集成 (EasyMDE):**
    *   [x] 在 `post_edit.html` 中，使用 Alpine.js (`x-data`, `x-init`) 来初始化 EasyMDE 并将其绑定到 `<textarea>` 上。

4.  **后台交互实现 (HTMX):**
    *   [x] 将登录表单的提交改造为 HTMX 请求 (`hx-post`, `hx-target`)。
    *   [x] 实现文章保存/更新的表单通过 HTMX 提交。
    *   [x] 实现文章列表中的“删除”按钮通过 HTMX (`hx-delete`) 直接与后端交互并刷新列表 (`hx-swap`)。

5.  **图片上传功能 (Cloudflare R2):**
    *   [x] 在 `internal/service` 中封装一个 `FileUploader` 服务，用于处理文件上传到 R2 的逻辑。
    *   [x] 在 `internal/handler` 中创建一个接收图片上传的专用接口。
    *   [x] 配置 EasyMDE 的图片上传选项，使其调用该接口。

---

## Phase 3: 公众的橱窗 (Public Frontend)

**目标:** 构建面向访客的前端界面，展示博客文章和简历。

1.  **前端页面开发 (HTML + Tailwind CSS):**
    *   [x] 创建前端主布局 `web/template/frontend/layout.html`。
    *   [x] `index.html`: 网站首页，展示文章列表。
    *   [x] `post_detail.html`: 文章详情页。
    *   [x] `about.html`: “关于我”简历页。
    *   [x] `tag_list.html`: 按标签分类的文章列表页。

2.  **后端渲染逻辑:**
    *   [x] 在 `internal/handler` 中创建渲染上述前端页面的 Handlers。
    *   [x] 在 `internal/service` 中实现从数据库查询文章、标签等数据的逻辑。

3.  **功能实现与优化:**
    *   [x] **阅读时长估算:** 在 `post` service 中添加一个函数，在文章创建/更新时根据字数计算阅读时长并存入数据库。
    *   [x] **基础 SEO:** 在 `post_detail.html` 的模板中，动态渲染 `<title>` 和 `<meta name="description">` 标签。

---

## Phase 4: 注入灵魂 (AI Integration)

**目标:** 集成核心的 AI 亮点功能，展示技术实力。

1.  **AI 服务封装:**
    *   [x] 在 `internal/service` 中创建 `ai_service.go`。
    *   [x] 封装一个函数，通过标准 HTTP 请求调用 Gemini API，入参为文章内容，返回文章摘要。

2.  **AI 功能集成:**
    *   [x] 修改文章保存的业务逻辑：当文章状态为“已发布”时，异步触发 AI 摘要生成函数。
    *   [x] 将生成的摘要保存到文章表的 `summary` 字段。

3.  **前端展示:**
    *   [x] 在首页文章列表 (`index.html`) 的每篇文章卡片上，展示 AI 生成的摘要。
    *   [x] 将 AI 摘要也用于文章详情页的 `meta description` 标签中，优化 SEO。

---

## Phase 5: 部署上线 (Fly.io)

**目标:** 将应用部署到线上，实现零成本运维。

1.  **Dockerfile 编写:**
    *   [x] 创建 `Dockerfile`，使用多阶段构建：
        *   `node` 镜像作为 `builder` 阶段，用于 `npm run build`。
        *   `golang` 镜像作为 `compiler` 阶段，用于编译 Go 应用。
        *   `debian` 或 `alpine` 作为最终镜像，仅复制编译后的 Go 二进制文件、`web` 目录和 `migrations` (如有)。

2.  **部署配置:**
    *   [x] 安装 `flyctl` CLI 工具。
    *   [x] 在项目根目录执行 `fly launch`，生成 `fly.toml`。
    *   [x] 在 `fly.toml` 中配置一个持久化卷 (Volume)，并将其挂载到存放 SQLite 数据库文件的路径。

3.  **部署与验证:**
    *   [x] 执行 `fly deploy` 进行部署。
    *   [x] 访问 `fly.dev` 提供的域名，验证网站是否正常运行。
    *   [x] (可选) 配置 Cloudflare CDN 以获得更好的全球访问速度和免费 SSL。