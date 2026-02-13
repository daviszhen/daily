-- 重建数据库
DROP DATABASE IF EXISTS smart_daily;
CREATE DATABASE smart_daily;
USE smart_daily;

CREATE TABLE members (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    name VARCHAR(50) NOT NULL,
    avatar VARCHAR(255) DEFAULT '',
    role VARCHAR(50) DEFAULT '开发工程师',
    team VARCHAR(50) DEFAULT '研发团队'
);

CREATE TABLE daily_entries (
    id INT AUTO_INCREMENT PRIMARY KEY,
    member_id INT NOT NULL,
    daily_date DATE NOT NULL,
    content TEXT NOT NULL,
    summary TEXT,
    source VARCHAR(20) DEFAULT 'chat',
    created_at DATETIME DEFAULT NOW()
);

CREATE TABLE daily_summaries (
    id INT AUTO_INCREMENT PRIMARY KEY,
    member_id INT NOT NULL,
    daily_date DATE NOT NULL,
    summary TEXT,
    status TEXT,
    risk TEXT,
    blocker TEXT,
    UNIQUE KEY uk_member_date (member_id, daily_date)
);

-- 预设用户 密码都是 123456
INSERT INTO members (username, password, name, role) VALUES
('pengzhen',    '$2a$10$sH3qZ9F0SIrCWpcOi9oWDO6EjbWMRs4X/8d35hphzkYRRM.ESRsa.', '彭振',   '开发工程师'),
('caokai',      '$2a$10$sH3qZ9F0SIrCWpcOi9oWDO6EjbWMRs4X/8d35hphzkYRRM.ESRsa.', '曹凯',   '开发工程师'),
('zhaogangyi',  '$2a$10$sH3qZ9F0SIrCWpcOi9oWDO6EjbWMRs4X/8d35hphzkYRRM.ESRsa.', '赵刚毅', '开发工程师'),
('lifangfei',   '$2a$10$sH3qZ9F0SIrCWpcOi9oWDO6EjbWMRs4X/8d35hphzkYRRM.ESRsa.', '李芳菲', '开发工程师'),
('kuaiweikang', '$2a$10$sH3qZ9F0SIrCWpcOi9oWDO6EjbWMRs4X/8d35hphzkYRRM.ESRsa.', '蒯伟康', '开发工程师');
