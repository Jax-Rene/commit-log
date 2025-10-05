# CommitLog: AI 驱动的个人成长与展示平台

本项目是一个集专业博客、个人成长追踪与动态在线简历于一体的现代化个人品牌网站。旨在成为作者作为 AI 全栈工程师的核心作品集，全面展示其在产品设计、前后端开发、AI 功能集成等方面的综合能力。

---

## 技术栈

- **后端:** Go + Gin
- **前端:** HTMX + Alpine.js + Tailwind CSS
- **数据库:** SQLite + GORM
- **Markdown 编辑器:** Milkdown
- **AI 集成:** Gemini API
- **部署:** Docker + Fly.io

---

## 项目结构说明

```
.
├── cmd/                    # Go 应用的入口
│   └── server/             # Web 服务器
│       └── main.go         # 主程序，负责启动服务
├── internal/               # 项目私有核心代码
│   ├── db/                 # 数据库相关 (模型、连接、迁移)
│   ├── handler/            # Gin 的 HTTP Handlers (请求处理器)
│   ├── router/             # 路由定义
│   └── service/            # 核心业务逻辑
├── web/                    # 前端相关资源
│   ├── static/             # 静态文件 (CSS, JS, 图片)
│   └── template/           # Go HTML 模板
├── prd/                    # 产品相关文档 (PRD, Tech Spec)
├── go.mod                  # Go 模块依赖管理
├── package.json            # Node.js 依赖管理 (用于前端工具链)
├── tailwind.config.js      # Tailwind CSS 配置文件
├── development_plan.md     # 项目开发计划
├── commitlog.db            # SQLite 数据库文件
└── README.md               # 就是你现在看到的文件
```

### 核心目录职责

- **`/cmd/server/main.go`**: 项目的唯一入口。它的职责很简单：初始化必要的服务（如数据库连接），设置好路由，然后启动 Web 服务器。

- **`/internal`**: 存放所有项目的内部代码。根据 Go 的规范，`internal` 目录下的包只能被该项目自身导入，而不能被外部项目导入，保证了代码的封装性。
    - **`/db`**: 负责所有与数据库交互的逻辑。包括 GORM 的数据模型 (`user.go`, `post.go` 等)，以及数据库的初始化和连接 (`db.go`)。
    - **`/handler`**: 存放直接处理 HTTP 请求的函数。每个 handler 对应一个或多个路由，负责解析请求、调用业务逻辑、并返回响应。
    - **`/router`**: 定义了整个应用的所有 URL 路由。它像一个交通枢纽，将不同的 URL 请求分发给 `/handler` 中对应的处理器。
    - **`/service`**: 存放核心的业务逻辑。Handler 应该保持精简，只做请求和响应的“传达”，而复杂的业务处理、数据整合等都应该放在 Service 层。

- **`/web`**: 所有与前端界面相关的资源都存放在这里。
    - **`/static`**: 存放编译后的静态资源，如 `output.css`，以及未来可能有的 JavaScript 文件和图片。这些文件会被直接提供给浏览器。
    - **`/template`**: 存放 Go 的 `html/template` 文件。后端会读取这些模板，填充动态数据，然后渲染成最终的 HTML 页面返回给用户。

- **`/prd`**: 存放产品需求文档、技术方案文档等，方便随时回顾项目的设计初衷和规划。

---

## 开发

1.  **启动后端服务:**
    ```bash
    go run cmd/server/main.go
    ```

2.  **编译前端样式:**
    ```bash
    npm run build:css
    ```
