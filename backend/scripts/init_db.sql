-- Go-Job MySQL initialization script
-- Usage:
--   mysql -u root -p < scripts/init_db.sql

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

CREATE DATABASE IF NOT EXISTS go_job
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE go_job;

-- Drop tables in dependency order for idempotent re-run
DROP TABLE IF EXISTS job_log;
DROP TABLE IF EXISTS job_executor_heartbeat;
DROP TABLE IF EXISTS job_info;
DROP TABLE IF EXISTS job_executor;

-- 1) Executor registry
CREATE TABLE job_executor (
    id INT PRIMARY KEY AUTO_INCREMENT COMMENT '执行器ID',
    app_name VARCHAR(100) NOT NULL COMMENT '执行器应用名（集群唯一）',
    name VARCHAR(100) NOT NULL COMMENT '执行器显示名',
    address_type TINYINT NOT NULL DEFAULT 0 COMMENT '地址类型：0-自动注册，1-手动配置',
    address_list VARCHAR(512) DEFAULT NULL COMMENT '手动配置地址列表，逗号分隔',
    status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：0-禁用，1-正常',
    creator VARCHAR(50) DEFAULT NULL COMMENT '创建人',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_app_name (app_name),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='执行器表';

-- 2) Job definitions
CREATE TABLE job_info (
    id INT PRIMARY KEY AUTO_INCREMENT COMMENT '任务ID',
    job_name VARCHAR(100) NOT NULL COMMENT '任务名称',
    executor_id INT NOT NULL COMMENT '关联执行器ID',
    executor_handler VARCHAR(200) NOT NULL COMMENT '执行器处理函数名',
    executor_param VARCHAR(512) DEFAULT NULL COMMENT '执行器参数',
    cron VARCHAR(50) NOT NULL COMMENT 'Cron表达式',
    shard_total TINYINT NOT NULL DEFAULT 1 COMMENT '分片总数：1-非分片任务，>1-分片任务',
    shard_param VARCHAR(256) DEFAULT NULL COMMENT '分片参数，逗号分隔',
    timeout INT NOT NULL DEFAULT 30 COMMENT '任务超时时间（秒）',
    retry_count TINYINT NOT NULL DEFAULT 0 COMMENT '失败重试次数',
    priority TINYINT NOT NULL DEFAULT 1 COMMENT '任务优先级：1-低，2-中，3-高',
    status TINYINT NOT NULL DEFAULT 0 COMMENT '任务状态：0-暂停，1-运行',
    creator VARCHAR(50) DEFAULT NULL COMMENT '创建人',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_job_info_executor_id
      FOREIGN KEY (executor_id) REFERENCES job_executor(id) ON DELETE CASCADE,
    KEY idx_executor_id (executor_id),
    KEY idx_status (status),
    KEY idx_cron (cron)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务配置表';

-- 3) Execution logs
CREATE TABLE job_log (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '日志ID',
    job_id INT NOT NULL COMMENT '关联任务ID',
    executor_id INT NOT NULL COMMENT '执行器ID',
    executor_address VARCHAR(100) NOT NULL COMMENT '执行器节点地址',
    shard_index TINYINT NOT NULL DEFAULT 0 COMMENT '分片索引：0-非分片任务，>0-分片索引',
    executor_param VARCHAR(512) DEFAULT NULL COMMENT '实际执行参数',
    trigger_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '任务触发时间',
    start_time DATETIME DEFAULT NULL COMMENT '任务开始执行时间',
    end_time DATETIME DEFAULT NULL COMMENT '任务结束执行时间',
    cost_time INT DEFAULT NULL COMMENT '执行耗时（毫秒）',
    status TINYINT NOT NULL COMMENT '执行状态：0-待执行，1-执行中，2-执行成功，3-执行失败',
    error_msg TEXT DEFAULT NULL COMMENT '失败错误信息',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_job_log_job_id
      FOREIGN KEY (job_id) REFERENCES job_info(id) ON DELETE CASCADE,
    CONSTRAINT fk_job_log_executor_id
      FOREIGN KEY (executor_id) REFERENCES job_executor(id) ON DELETE CASCADE,
    KEY idx_job_id (job_id),
    KEY idx_trigger_time (trigger_time),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务执行日志表';

-- 4) Executor heartbeats
CREATE TABLE job_executor_heartbeat (
    id INT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    executor_app_name VARCHAR(100) NOT NULL COMMENT '执行器应用名',
    executor_address VARCHAR(100) NOT NULL COMMENT '执行器节点地址',
    heartbeat_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最新心跳时间',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_app_address (executor_app_name, executor_address),
    KEY idx_heartbeat_time (heartbeat_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='执行器心跳表';

SET FOREIGN_KEY_CHECKS = 1;

-- ---------------------------------------------------------------------------
-- Seed data (for local development)
-- ---------------------------------------------------------------------------

INSERT INTO job_executor (app_name, name, address_type, address_list, status, creator)
VALUES
('data-sync-executor', '数据同步执行器', 0, NULL, 1, 'system'),
('report-executor', '报表执行器', 1, '127.0.0.1:9091,127.0.0.1:9092', 1, 'system');

INSERT INTO job_info (
    job_name, executor_id, executor_handler, executor_param, cron,
    shard_total, shard_param, timeout, retry_count, priority, status, creator
)
VALUES
('数据同步任务', 1, 'dataSyncHandler', 'source=mysql&target=redis', '0 0 * * *', 4, '0,1,2,3', 60, 2, 2, 1, 'system'),
('日报生成任务', 2, 'dailyReportHandler', 'biz=finance', '0 30 1 * *', 1, NULL, 120, 1, 3, 0, 'system');

INSERT INTO job_executor_heartbeat (executor_app_name, executor_address, heartbeat_time)
VALUES
('data-sync-executor', '127.0.0.1:9090', NOW()),
('report-executor', '127.0.0.1:9091', NOW());

INSERT INTO job_log (
    job_id, executor_id, executor_address, shard_index, executor_param,
    trigger_time, start_time, end_time, cost_time, status, error_msg
)
VALUES
(1, 1, '127.0.0.1:9090', 0, 'source=mysql&target=redis', NOW(), NOW(), NOW(), 1450, 2, NULL),
(1, 1, '127.0.0.1:9090', 1, 'source=mysql&target=redis', NOW(), NOW(), NOW(), 1520, 2, NULL),
(2, 2, '127.0.0.1:9091', 0, 'biz=finance', NOW(), NOW(), NOW(), 2150, 3, 'upstream timeout');
