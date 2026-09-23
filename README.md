# AgriDispatch（农机调度管理系统）

> 项目类型：全栈Web应用

面向农业合作社与种植大户的农机作业调度平台，支持农机资源管理、作业任务指派（推荐空闲农机/驾驶员）、实时地图轨迹监控（WebSocket + 高德地图）、作业记录统计报表、维修保养提醒与驾驶员管理。

## 快速启动（Docker Compose 一键部署，首选）

```bash
# 1. 首次启动前复制环境变量
cp .env.example .env

# 2. 启动全部服务（MySQL + Redis + 后端 + 前端）
docker compose up -d

# 3. 查看健康状态
docker compose ps
```

访问地址：

- 前端调度看板：http://localhost:18621
- 后端 API：http://localhost:19621
- 健康检查：http://localhost:19621/healthz
- Swagger：http://localhost:19621/docs/index.html

演示账号：`admin / admin123`

## 本地开发

```bash
# 前端（Vue 3 + TS + Vite + Element Plus + ECharts）
cd frontend
npm install
npm run dev        # http://localhost:18621

# 后端（Go）
cd backend
go mod tidy
go run ./cmd/server
```

## 技术栈

| 层次 | 技术 |
| --- | --- |
| 前端 | Vue 3 + TypeScript + Vite + Element Plus + ECharts |
| 地图 | 高德地图 API（JSAPI） |
| 后端 | Go 1.22 + Gin + GORM |
| 数据库 | MySQL 8.0 |
| 缓存 | Redis 7（go-redis/v9，看板缓存 + 实时推送） |
| 实时通信 | WebSocket（gorilla/websocket，农机实时位置推送） |
| 认证 | JWT (github.com/golang-jwt/jwt/v5) + RBAC |
| 配置 | github.com/caarlos0/env/v11 |
| 日志 | Go 标准库 log/slog |

## 项目目录结构

```
.
├── docker-compose.yml          # db + redis + backend + frontend
├── .env / .env.example
├── README.md
├── frontend/
│   ├── Dockerfile / nginx.conf # /api/ → backend:8080/api/v1/，/ws → backend:8080/ws
│   └── src/
│       ├── features/           # DashboardView
│       ├── components/         # MachineTable/TaskBoard/MapTrackPanel/...
│       ├── services/           # API 调用（overview/dispatch）
│       ├── stores/  types/  constants/  logger/  errors/
└── backend/
    ├── Dockerfile
    ├── database/init.sql
    ├── cmd/server/main.go
    └── internal/
        ├── config/  model/  repository/  service/  handler/
        ├── router/  middleware/  ws/          # WebSocket Hub
        ├── constants/  errors/  logger/  util/  database/
```

## 主要 API 列表

统一前缀 `/api/v1`，统一响应 `{ "code": 0, "message": "ok", "data": ... }`。

| 方法 | 路径 | 说明 | 鉴权 |
| --- | --- | --- | --- |
| POST | /auth/login | 登录 | - |
| GET | /auth/me | 当前用户 | JWT |
| GET | /dashboard/overview | 调度看板总览（农机/任务/轨迹/统计/保养/工单/费用/驾驶员） | - |
| POST | /tasks/:id/dispatch | 一键派单（推荐空闲农机与驾驶员） | - |
| GET | /dashboard/reports/work/export | 作业报表导出信息 | - |
| POST | /maintenance/reminders/:id/orders | 保养开单（计划日期、维修点；作业中/已有未结束工单则拒绝并记录原因） | - |
| POST | /maintenance/orders/:id/complete | 工单完工（工时、费用、下次剩余小时；事务生效，重复完工不产生第二笔费用） | - |
| POST | /maintenance/orders/:id/cancel | 取消未完工工单（释放农机、提醒恢复待开单） | - |
| GET | /ws | WebSocket 实时轨迹推送 | - |
| GET | /healthz | 健康检查（DB + Redis） | - |

### 保养工单流程

1. **开单**：维修工期由口头排期改为工单排期。在保养提醒上填写`计划日期`与`维修点`；若农机`作业中`或该农机`已有未结束工单`，接口落一条`已拒绝`工单并返回拒绝原因（页面可见），提醒与农机状态不变。
2. **成功开单**：提醒节点变为`处理中`、农机变为`维修中`，工单状态`处理中`。
3. **完工**：填写`实际工时`、`费用`、`下次剩余小时`；工单置`已完工`、生成一笔费用记录、提醒节点置`已完工`并刷新剩余小时与预警等级、农机恢复`空闲`——四项在同一数据库事务中生效。重复完工幂等返回，不再生成第二笔费用。
4. **取消**：仅未完工工单可取消，取消后农机释放为`空闲`、提醒恢复为`待开单`，可重新开单。

## 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| COMPOSE_PROJECT_NAME | Compose 项目名 | agridispatch |
| APP_ENV | 运行环境 | development |
| SERVER_PORT | 后端端口（容器内） | 8080 |
| DB_HOST / DB_PORT | 数据库地址/端口 | db / 3306 |
| DB_NAME / DB_USER / DB_PASSWORD | 数据库配置 | agridispatch |
| DB_ROOT_PASSWORD | MySQL root 密码 | agridispatch_root_pwd |
| REDIS_HOST / REDIS_PORT | Redis 地址/端口 | redis / 6379 |
| REDIS_PASSWORD | Redis 密码 | 空 |
| JWT_SECRET | JWT 密钥 | 请修改 |
| FRONTEND_PORT | 前端宿主端口 | 18621 |
| BACKEND_PORT | 后端宿主端口 | 19621 |
| DB_PORT | MySQL 宿主端口 | 33321 |
| REDIS_PORT | Redis 宿主端口 | 36321 |

## Docker 部署说明

- 端口映射：前端 `18621:80`、后端 `19621:8080`、MySQL `33321:3306`、Redis `36321:6379`
- 数据卷：`db_data`、`redis_data`
- Nginx 反代：`/api/ → http://backend:8080/api/v1/`；`/ws` 配置 WebSocket Upgrade/Connection 头
- 常见问题：等待 `docker compose ps` 全部 healthy（MySQL 初始化约 30~120 秒）；高德地图 Key 在线上环境需按 `MapTrackPanel` 接入真实 JSAPI Key

## License

MIT License
