# Go-Job Backend

Go-Job 是一个基于 Go 的分布式任务调度平台后端（调度中心 + 执行器），当前仓库为后端工程骨架，已集成：

- Gin（HTTP 服务）
- GORM（MySQL）
- Redis Client
- Viper（配置管理）
- Zap（日志）
- Swagger（API 文档入口）

## 目录结构

```text
backend/
├── cmd/
│   ├── server/        # 调度中心入口
│   └── executor/      # 执行器入口
├── config/            # 配置文件
├── internal/          # 业务代码（后续按分层补充）
├── pkg/               # 公共组件封装
├── scripts/           # SQL / 部署脚本
├── api/swagger/       # Swagger 文档注册
└── docs/              # 设计与部署文档
```

## 环境要求

- Go 1.22+
- MySQL 8.0+
- Redis 7.0+

## 快速开始

### 1) 初始化数据库

在项目根目录执行（Windows PowerShell）：

```powershell
mysql -u root -p < backend/scripts/init_db.sql
```

### 2) 启动调度中心

```powershell
cd backend
go run cmd/server/main.go
```

默认地址：

- Health: `http://localhost:8080/health`
- Swagger: `http://localhost:8080/swagger/index.html`

### 3) 启动执行器

新开一个终端：

```powershell
cd backend
go run cmd/executor/main.go
```

默认地址：

- Health: `http://localhost:9090/health`
- Ping: `http://localhost:9090/api/v1/executor/ping`

## 配置说明

- 调度中心配置：`config/dev.yaml`
- 执行器配置：`config/executor.yaml`
- 生产配置：`config/prod.yaml`

当前默认本地配置：

- MySQL: `127.0.0.1:3306` / `root` / `123456` / `go_job`
- Redis: `127.0.0.1:6379`

## 可用环境变量

- `GO_JOB_CONFIG`：覆盖调度中心配置路径（默认 `config/dev.yaml`）
- `GO_JOB_EXECUTOR_CONFIG`：覆盖执行器配置路径（默认 `config/executor.yaml`）

## 开发说明

- 当前已完成基础框架和组件接入，`internal` 业务模块处于骨架阶段。
- 如需全量编译通过，请先补齐空占位 `.go` 文件（或最小 package 声明）。

## License

MIT
