-- ==============================================================================
-- EXAM SHIELD DATABASE SCHEMA & SEED DATA (MySQL 5.7+ / 8.0+ / MariaDB)
-- Built by Awodele Kehinde
-- ==============================================================================

CREATE DATABASE IF NOT EXISTS `examshield` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `examshield`;

SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------------------------
-- 1. Table: organizations
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `organizations`;
CREATE TABLE `organizations` (
  `id` CHAR(36) NOT NULL,
  `name` VARCHAR(255) NOT NULL,
  `code` VARCHAR(100) NOT NULL,
  `active` TINYINT(1) DEFAULT 1,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_organizations_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 2. Table: users
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `users`;
CREATE TABLE `users` (
  `id` CHAR(36) NOT NULL,
  `organization_id` CHAR(36) NOT NULL,
  `email` VARCHAR(191) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `full_name` VARCHAR(255) NOT NULL,
  `role` VARCHAR(20) NOT NULL,
  `active` TINYINT(1) DEFAULT 1,
  `suspended` TINYINT(1) DEFAULT 0,
  `last_login_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_email` (`email`),
  KEY `idx_users_organization_id` (`organization_id`),
  KEY `idx_users_role` (`role`),
  KEY `idx_users_deleted_at` (`deleted_at`),
  CONSTRAINT `fk_users_organization` FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 3. Table: students
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `students`;
CREATE TABLE `students` (
  `id` CHAR(36) NOT NULL,
  `user_id` CHAR(36) NOT NULL,
  `matric_number` VARCHAR(100) DEFAULT NULL,
  `department` VARCHAR(150) DEFAULT NULL,
  `reference_face_id` CHAR(36) DEFAULT NULL,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_students_user_id` (`user_id`),
  UNIQUE KEY `idx_students_matric_number` (`matric_number`),
  CONSTRAINT `fk_students_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 4. Table: lecturers
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `lecturers`;
CREATE TABLE `lecturers` (
  `id` CHAR(36) NOT NULL,
  `user_id` CHAR(36) NOT NULL,
  `staff_number` VARCHAR(100) DEFAULT NULL,
  `department` VARCHAR(150) DEFAULT NULL,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_lecturers_user_id` (`user_id`),
  CONSTRAINT `fk_lecturers_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 5. Table: administrators
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `administrators`;
CREATE TABLE `administrators` (
  `id` CHAR(36) NOT NULL,
  `user_id` CHAR(36) NOT NULL,
  `title` VARCHAR(100) DEFAULT NULL,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_administrators_user_id` (`user_id`),
  CONSTRAINT `fk_administrators_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 6. Table: refresh_tokens
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `refresh_tokens`;
CREATE TABLE `refresh_tokens` (
  `id` CHAR(36) NOT NULL,
  `user_id` CHAR(36) NOT NULL,
  `token_hash` VARCHAR(255) NOT NULL,
  `expires_at` DATETIME(3) NOT NULL,
  `revoked` TINYINT(1) DEFAULT 0,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_refresh_tokens_token_hash` (`token_hash`),
  KEY `idx_refresh_tokens_user_id` (`user_id`),
  CONSTRAINT `fk_refresh_tokens_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 7. Table: exams
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `exams`;
CREATE TABLE `exams` (
  `id` CHAR(36) NOT NULL,
  `organization_id` CHAR(36) NOT NULL,
  `created_by_user_id` CHAR(36) NOT NULL,
  `title` VARCHAR(255) NOT NULL,
  `course` VARCHAR(255) DEFAULT NULL,
  `course_code` VARCHAR(100) DEFAULT NULL,
  `duration_minutes` INT NOT NULL,
  `start_at` DATETIME(3) DEFAULT NULL,
  `end_at` DATETIME(3) DEFAULT NULL,
  `mode` VARCHAR(20) NOT NULL DEFAULT 'ONLINE',
  `status` VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
  `randomize_questions` TINYINT(1) DEFAULT 0,
  `randomize_answers` TINYINT(1) DEFAULT 0,
  `allow_navigation` TINYINT(1) DEFAULT 1,
  `enable_ai_monitoring` TINYINT(1) DEFAULT 1,
  `require_face_verification` TINYINT(1) DEFAULT 1,
  `passing_score` DOUBLE DEFAULT 0,
  `instructions` TEXT DEFAULT NULL,
  `lan_server_ip` VARCHAR(45) DEFAULT NULL,
  `lan_server_port` INT DEFAULT NULL,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_exams_organization_id` (`organization_id`),
  KEY `idx_exams_created_by_user_id` (`created_by_user_id`),
  KEY `idx_exams_status` (`status`),
  KEY `idx_exams_deleted_at` (`deleted_at`),
  CONSTRAINT `fk_exams_organization` FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_exams_creator` FOREIGN KEY (`created_by_user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 8. Table: questions
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `questions`;
CREATE TABLE `questions` (
  `id` CHAR(36) NOT NULL,
  `exam_id` CHAR(36) NOT NULL,
  `type` VARCHAR(30) NOT NULL,
  `text` TEXT NOT NULL,
  `points` DOUBLE DEFAULT 1,
  `order_index` INT DEFAULT 0,
  `correct_text` TEXT DEFAULT NULL,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_questions_exam_id` (`exam_id`),
  CONSTRAINT `fk_questions_exam` FOREIGN KEY (`exam_id`) REFERENCES `exams` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 9. Table: options
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `options`;
CREATE TABLE `options` (
  `id` CHAR(36) NOT NULL,
  `question_id` CHAR(36) NOT NULL,
  `text` TEXT NOT NULL,
  `is_correct` TINYINT(1) DEFAULT 0,
  `order_index` INT DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_options_question_id` (`question_id`),
  CONSTRAINT `fk_options_question` FOREIGN KEY (`question_id`) REFERENCES `questions` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 10. Table: exam_attempts
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `exam_attempts`;
CREATE TABLE `exam_attempts` (
  `id` CHAR(36) NOT NULL,
  `exam_id` CHAR(36) NOT NULL,
  `student_id` CHAR(36) NOT NULL,
  `status` VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED',
  `verified_face` TINYINT(1) DEFAULT 0,
  `started_at` DATETIME(3) NULL,
  `submitted_at` DATETIME(3) NULL,
  `score` DOUBLE NULL,
  `ip_address` VARCHAR(45) DEFAULT NULL,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_exam_student` (`exam_id`, `student_id`),
  KEY `idx_exam_attempts_status` (`status`),
  CONSTRAINT `fk_exam_attempts_exam` FOREIGN KEY (`exam_id`) REFERENCES `exams` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_exam_attempts_student` FOREIGN KEY (`student_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 11. Table: answers
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `answers`;
CREATE TABLE `answers` (
  `id` CHAR(36) NOT NULL,
  `attempt_id` CHAR(36) NOT NULL,
  `question_id` CHAR(36) NOT NULL,
  `selected_option_id` CHAR(36) DEFAULT NULL,
  `text_answer` TEXT DEFAULT NULL,
  `is_correct` TINYINT(1) DEFAULT NULL,
  `awarded_points` DOUBLE DEFAULT 0,
  `answered_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_attempt_question` (`attempt_id`, `question_id`),
  KEY `idx_answers_selected_option_id` (`selected_option_id`),
  CONSTRAINT `fk_answers_attempt` FOREIGN KEY (`attempt_id`) REFERENCES `exam_attempts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_answers_question` FOREIGN KEY (`question_id`) REFERENCES `questions` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 12. Table: face_verifications
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `face_verifications`;
CREATE TABLE `face_verifications` (
  `id` CHAR(36) NOT NULL,
  `student_id` CHAR(36) NOT NULL,
  `exam_id` CHAR(36) DEFAULT NULL,
  `attempt_id` CHAR(36) DEFAULT NULL,
  `matched` TINYINT(1) DEFAULT 0,
  `confidence` DOUBLE DEFAULT 0,
  `is_reference` TINYINT(1) DEFAULT 0,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_face_verifications_student_id` (`student_id`),
  KEY `idx_face_verifications_exam_id` (`exam_id`),
  KEY `idx_face_verifications_attempt_id` (`attempt_id`),
  CONSTRAINT `fk_face_verifications_student` FOREIGN KEY (`student_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 13. Table: monitoring_sessions
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `monitoring_sessions`;
CREATE TABLE `monitoring_sessions` (
  `id` CHAR(36) NOT NULL,
  `attempt_id` CHAR(36) NOT NULL,
  `student_id` CHAR(36) NOT NULL,
  `exam_id` CHAR(36) NOT NULL,
  `camera_active` TINYINT(1) DEFAULT 0,
  `connection_quality` VARCHAR(20) DEFAULT 'UNKNOWN',
  `started_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `ended_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_monitoring_sessions_attempt_id` (`attempt_id`),
  KEY `idx_monitoring_sessions_student_id` (`student_id`),
  KEY `idx_monitoring_sessions_exam_id` (`exam_id`),
  CONSTRAINT `fk_monitoring_sessions_attempt` FOREIGN KEY (`attempt_id`) REFERENCES `exam_attempts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 14. Table: ai_events
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `ai_events`;
CREATE TABLE `ai_events` (
  `id` CHAR(36) NOT NULL,
  `attempt_id` CHAR(36) NOT NULL,
  `student_id` CHAR(36) NOT NULL,
  `exam_id` CHAR(36) NOT NULL,
  `event_type` VARCHAR(100) NOT NULL,
  `severity` VARCHAR(20) NOT NULL,
  `confidence` DOUBLE DEFAULT 0,
  `description` TEXT DEFAULT NULL,
  `reviewed` TINYINT(1) DEFAULT 0,
  `reviewed_by` CHAR(36) DEFAULT NULL,
  `timestamp` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_ai_events_attempt_id` (`attempt_id`),
  KEY `idx_ai_events_student_id` (`student_id`),
  KEY `idx_ai_events_exam_id` (`exam_id`),
  KEY `idx_ai_events_event_type` (`event_type`),
  KEY `idx_ai_events_severity` (`severity`),
  KEY `idx_ai_events_timestamp` (`timestamp`),
  CONSTRAINT `fk_ai_events_attempt` FOREIGN KEY (`attempt_id`) REFERENCES `exam_attempts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 15. Table: security_events
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `security_events`;
CREATE TABLE `security_events` (
  `id` CHAR(36) NOT NULL,
  `user_id` CHAR(36) DEFAULT NULL,
  `exam_id` CHAR(36) DEFAULT NULL,
  `event_type` VARCHAR(100) NOT NULL,
  `severity` VARCHAR(20) NOT NULL,
  `description` TEXT DEFAULT NULL,
  `ip_address` VARCHAR(45) DEFAULT NULL,
  `timestamp` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_security_events_user_id` (`user_id`),
  KEY `idx_security_events_exam_id` (`exam_id`),
  KEY `idx_security_events_timestamp` (`timestamp`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------------------------
-- 16. Table: audit_logs
-- ------------------------------------------------------------------------------
DROP TABLE IF EXISTS `audit_logs`;
CREATE TABLE `audit_logs` (
  `id` CHAR(36) NOT NULL,
  `actor_id` CHAR(36) NOT NULL,
  `actor_role` VARCHAR(30) DEFAULT NULL,
  `action` VARCHAR(100) NOT NULL,
  `resource` VARCHAR(100) DEFAULT NULL,
  `resource_id` VARCHAR(100) DEFAULT NULL,
  `ip_address` VARCHAR(45) DEFAULT NULL,
  `metadata` TEXT DEFAULT NULL,
  `timestamp` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_audit_logs_actor_id` (`actor_id`),
  KEY `idx_audit_logs_actor_role` (`actor_role`),
  KEY `idx_audit_logs_action` (`action`),
  KEY `idx_audit_logs_resource` (`resource`),
  KEY `idx_audit_logs_resource_id` (`resource_id`),
  KEY `idx_audit_logs_timestamp` (`timestamp`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;

-- ==============================================================================
-- INITIAL SEED DATA
-- Default Credentials:
-- 1. Super Admin: admin@examshield.edu     / Password: Admin@123456
-- 2. Lecturer:    lecturer@examshield.edu  / Password: Lecturer@123456
-- 3. Student:     student@examshield.edu   / Password: Student@123456
-- ==============================================================================

-- 1. Seed Organization
INSERT INTO `organizations` (`id`, `name`, `code`, `active`, `created_at`, `updated_at`) VALUES
('a0000000-0000-0000-0000-000000000001', 'Apex Institute of Technology', 'APEX-TECH', 1, NOW(3), NOW(3));

-- 2. Seed Users
INSERT INTO `users` (`id`, `organization_id`, `email`, `password_hash`, `full_name`, `role`, `active`, `suspended`, `created_at`, `updated_at`) VALUES
('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'admin@examshield.edu', '$2a$12$CEA4.cziHKfoDexdhmRU3usbVNyA8RK89BEX0sA/Rqb22gohF0VtC', 'Awodele Kehinde (Chief Proctor)', 'SUPER_ADMIN', 1, 0, NOW(3), NOW(3)),
('b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'lecturer@examshield.edu', '$2a$12$KtlLscguoLd69Kjyzjtq7ewOnssOofzzt67rPq/K4p8MhWbe5xtV6', 'Dr. Sarah Johnson', 'LECTURER', 1, 0, NOW(3), NOW(3)),
('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 'student@examshield.edu', '$2a$12$PTaIwNtpK8JqrtiRUkxGPO61K3MdBi.DHYYWe3b3ZVLo/1q/LoKeu', 'Michael Adeyemi', 'STUDENT', 1, 0, NOW(3), NOW(3));

-- 3. Seed Role Profiles
INSERT INTO `administrators` (`id`, `user_id`, `title`, `created_at`, `updated_at`) VALUES
('c0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'Director of Academic Integrity', NOW(3), NOW(3));

INSERT INTO `lecturers` (`id`, `user_id`, `staff_number`, `department`, `created_at`, `updated_at`) VALUES
('c0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000002', 'LEC/2026/042', 'Computer Science & Engineering', NOW(3), NOW(3));

INSERT INTO `students` (`id`, `user_id`, `matric_number`, `department`, `created_at`, `updated_at`) VALUES
('c0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000003', 'CSC/2023/1089', 'Computer Science & Engineering', NOW(3), NOW(3));

-- 4. Seed Sample Exam
INSERT INTO `exams` (`id`, `organization_id`, `created_by_user_id`, `title`, `course`, `course_code`, `duration_minutes`, `start_at`, `end_at`, `mode`, `status`, `randomize_questions`, `randomize_answers`, `allow_navigation`, `enable_ai_monitoring`, `require_face_verification`, `passing_score`, `instructions`, `created_at`, `updated_at`) VALUES
('d0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'Advanced Distributed Systems & Integrity Protocols', 'Distributed Systems', 'CSC401', 60, NOW(3), DATE_ADD(NOW(3), INTERVAL 7 DAY), 'ONLINE', 'PUBLISHED', 1, 1, 1, 1, 1, 50.0, 'Welcome to the CSC401 examination. Ensure continuous webcam visibility throughout the session. Any background movement, secondary person presence, or tab switching will be flagged to proctors.', NOW(3), NOW(3));

-- 5. Seed Questions
INSERT INTO `questions` (`id`, `exam_id`, `type`, `text`, `points`, `order_index`, `correct_text`, `created_at`, `updated_at`) VALUES
('e0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000001', 'MULTIPLE_CHOICE', 'Which of the following database engines provides native ACID transactions with InnoDB storage engine?', 5.0, 1, NULL, NOW(3), NOW(3)),
('e0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000001', 'MULTIPLE_CHOICE', 'In Exam Shield architecture, how does the frontend communicate real-time proctoring telemetry to the backend?', 5.0, 2, NULL, NOW(3), NOW(3)),
('e0000000-0000-0000-0000-000000000003', 'd0000000-0000-0000-0000-000000000001', 'TRUE_FALSE', 'Exam Shield stores raw video recordings of candidates in the database.', 5.0, 3, 'false', NOW(3), NOW(3));

-- 6. Seed Options for Questions
INSERT INTO `options` (`id`, `question_id`, `text`, `is_correct`, `order_index`) VALUES
('f0000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000001', 'MySQL', 1, 1),
('f0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000001', 'Redis Cache', 0, 2),
('f0000000-0000-0000-0000-000000000003', 'e0000000-0000-0000-0000-000000000001', 'Memcached', 0, 3),
('f0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'Flat File CSV', 0, 4),
('f0000000-0000-0000-0000-000000000005', 'e0000000-0000-0000-0000-000000000002', 'WebSockets (ws://) with multiplexed JSON events', 1, 1),
('f0000000-0000-0000-0000-000000000006', 'e0000000-0000-0000-0000-000000000002', 'Direct SMTP Email Triggers', 0, 2),
('f0000000-0000-0000-0000-000000000007', 'e0000000-0000-0000-0000-000000000002', 'FTP Polling loop', 0, 3),
('f0000000-0000-0000-0000-000000000008', 'e0000000-0000-0000-0000-000000000002', 'IRC Bot notifications', 0, 4);
