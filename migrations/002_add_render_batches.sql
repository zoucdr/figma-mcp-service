-- 渲染任务组表
CREATE TABLE IF NOT EXISTS figma_render_batches (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    project_ids TEXT NOT NULL,  -- JSON数组: [1,2,3]，支持多项目批量渲染
    file_key VARCHAR(100) NOT NULL,
    format VARCHAR(10) NOT NULL,
    scale DOUBLE NOT NULL,
    status VARCHAR(20) DEFAULT 'waiting',  -- waiting/processing/completed/partial/failed/cancelled
    total_queues INT DEFAULT 0,  -- 总队列数
    completed_queues INT DEFAULT 0,  -- 已完成队列数
    total_nodes INT DEFAULT 0,  -- 总节点数
    processed_nodes INT DEFAULT 0,  -- 已处理节点数
    failed_nodes INT DEFAULT 0,  -- 失败节点数
    progress INT DEFAULT 0,  -- 0-100
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 索引
CREATE INDEX idx_render_batches_user ON figma_render_batches(user_id);
CREATE INDEX idx_render_batches_status ON figma_render_batches(status);
CREATE INDEX idx_render_batches_file_key ON figma_render_batches(file_key);

-- 队列-批次关联表
CREATE TABLE IF NOT EXISTS figma_queue_batch_relations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    queue_id INT NOT NULL,
    batch_id INT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_queue_batch (queue_id, batch_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_queue_batch_queue ON figma_queue_batch_relations(queue_id);
CREATE INDEX idx_queue_batch_batch ON figma_queue_batch_relations(batch_id);

-- 修改项目表，添加活跃批次ID字段（如果不存在）
-- MySQL 不支持 ADD COLUMN IF NOT EXISTS，需要使用存储过程或手动检查
ALTER TABLE figma_projects ADD COLUMN active_render_batch_id INT DEFAULT 0;

-- 创建索引
CREATE INDEX idx_projects_active_batch ON figma_projects(active_render_batch_id);


