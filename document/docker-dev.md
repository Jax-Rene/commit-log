# Docker 化的本地调试流程

为了在本地尽量贴近 Fly.io 的运行环境，我们提供了 `docker-compose.dev.yml`，通过容器运行同一份生产镜像。这样可以提前验证依赖安装、挂载卷及环境变量配置是否正确。

## 先决条件
- 已安装 Docker 与 Docker Compose v2 (`docker compose` 命令可用)。
- 仓库根目录存在 `tmp-data/`（第一次运行时会自动创建，用于映射应用中的 `/data` 持久化目录）。

## 常用命令
- 构建镜像：`make docker-build`
- 启动服务：`make docker-dev`
  - 服务监听 `http://localhost:8080`
  - 默认挂载 `./tmp-data` 到容器内 `/data`，与 Fly volume 一致
  - 如果需要预置管理员账号，请在命令前设置 `SUPER_ROOT_USER_NAME`、`SUPER_ROOT_PASSWORD`
- 停止并移除容器：`make docker-dev-down`

## 目录与环境变量映射
- `/data`：映射到宿主机 `./tmp-data`，存放 SQLite 数据库与上传文件
- `UPLOAD_DIR=/data/uploads`、`UPLOAD_URL_PATH=/uploads`：与生产保持一致
- 其他环境变量（如 `PORT`、`GIN_MODE`）已经在 compose 文件中设置为 Fly 默认值，可根据需要覆盖。

这样可以在容器里复现部署环境，又不影响现有的 `make dev` 快速迭代流程。
