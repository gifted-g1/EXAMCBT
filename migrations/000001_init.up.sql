-- Exam Shield initial schema (MySQL 8.x, InnoDB, utf8mb4).
-- All DATETIME values are stored in UTC (DSN uses loc=UTC).

CREATE TABLE IF NOT EXISTS users (
  id            BIGINT AUTO_INCREMENT PRIMARY KEY,
  username      VARCHAR(20)  NOT NULL,
  password_hash VARCHAR(72)  NOT NULL,
  full_name     VARCHAR(120) NOT NULL,
  role          ENUM('proctor','admin') NOT NULL,
  is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_users_username (username),
  INDEX idx_role (role),
  CONSTRAINT chk_users_username CHECK (REGEXP_LIKE(username, '^[0-9]{8}$'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS students (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  matric_no   VARCHAR(20)  NOT NULL,
  full_name   VARCHAR(120) NOT NULL,
  photo_url   VARCHAR(255) NULL,
  created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_students_matric (matric_no),
  CONSTRAINT chk_students_matric CHECK (REGEXP_LIKE(matric_no, '^[A-Z]{2,4}/[0-9]{2}/[0-9]{4}$', 'c'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS exams (
  id               BIGINT AUTO_INCREMENT PRIMARY KEY,
  title            VARCHAR(150) NOT NULL,
  description      TEXT NULL,
  created_by       BIGINT NOT NULL,
  starts_at        DATETIME(3) NOT NULL,
  ends_at          DATETIME(3) NOT NULL,
  status           ENUM('DRAFT','SCHEDULED','ACTIVE','CLOSED') NOT NULL DEFAULT 'DRAFT',
  lan_cidr         VARCHAR(43) NOT NULL,          -- 43 fits a full IPv6 CIDR
  pin_code         CHAR(4) NULL,
  media_purged_at  DATETIME(3) NULL,
  created_at       DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at       DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  CONSTRAINT fk_exams_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT chk_exams_window CHECK (ends_at > starts_at),
  INDEX idx_status_window (status, starts_at, ends_at),
  INDEX idx_exams_created_by (created_by)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS questions (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  exam_id         BIGINT NOT NULL,
  position        INT NOT NULL DEFAULT 0,
  kind            ENUM('MCQ','TEXT') NOT NULL DEFAULT 'MCQ',
  prompt          TEXT NOT NULL,
  options         JSON NULL,
  correct_option  INT NULL,
  marks           DECIMAL(5,2) NOT NULL DEFAULT 1,
  CONSTRAINT fk_questions_exam FOREIGN KEY (exam_id) REFERENCES exams(id) ON DELETE CASCADE,
  INDEX idx_exam_position (exam_id, position)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS exam_sessions (
  id               BIGINT AUTO_INCREMENT PRIMARY KEY,
  exam_id          BIGINT NOT NULL,
  student_id       BIGINT NOT NULL,
  device_fp        VARCHAR(64) NOT NULL,
  client_ip        VARCHAR(45) NOT NULL,
  started_at       DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  ended_at         DATETIME(3) NULL,
  last_seen_at     DATETIME(3) NULL,
  integrity_score  DECIMAL(5,2) NOT NULL DEFAULT 100,
  face_match_score DECIMAL(4,3) NULL,
  reference_key    VARCHAR(255) NULL,
  id_photo_key     VARCHAR(255) NULL,
  consent_at       DATETIME(3) NULL,
  consent_version  VARCHAR(16) NULL,
  score            DECIMAL(7,2) NULL,
  auto_submitted   BOOLEAN NOT NULL DEFAULT FALSE,
  status           ENUM('INIT','ACTIVE','SUBMITTED','CLOSED') NOT NULL DEFAULT 'INIT',
  UNIQUE KEY uq_exam_student (exam_id, student_id),
  CONSTRAINT fk_sessions_exam FOREIGN KEY (exam_id) REFERENCES exams(id) ON DELETE CASCADE,
  CONSTRAINT fk_sessions_student FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
  INDEX idx_status (status),
  INDEX idx_sessions_student (student_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS violations (
  id             BIGINT AUTO_INCREMENT PRIMARY KEY,
  session_id     BIGINT NOT NULL,
  event_type     VARCHAR(50) NOT NULL,
  severity       ENUM('LOW','MEDIUM','HIGH','CRITICAL') NOT NULL,
  source         ENUM('CLIENT','AI','PROCTOR') NOT NULL DEFAULT 'CLIENT',
  occurred_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  media_ref      VARCHAR(255) NULL,
  ai_confidence  DECIMAL(4,3) NULL,
  metadata       JSON NULL,
  review_status  ENUM('PENDING','CONFIRMED','DISMISSED') NOT NULL DEFAULT 'PENDING',
  reviewed_by    BIGINT NULL,
  reviewed_at    DATETIME(3) NULL,
  CONSTRAINT fk_violations_session FOREIGN KEY (session_id) REFERENCES exam_sessions(id) ON DELETE CASCADE,
  CONSTRAINT fk_violations_reviewer FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL,
  INDEX idx_session_time (session_id, occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ai_findings (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  session_id  BIGINT NOT NULL,
  model_name  VARCHAR(50) NOT NULL,
  inference   JSON NOT NULL,
  created_at  DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  CONSTRAINT fk_findings_session FOREIGN KEY (session_id) REFERENCES exam_sessions(id) ON DELETE CASCADE,
  INDEX idx_session_model (session_id, model_name),
  INDEX idx_findings_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS answers (
  id           BIGINT AUTO_INCREMENT PRIMARY KEY,
  session_id   BIGINT NOT NULL,
  question_id  BIGINT NOT NULL,
  response     TEXT NULL,
  saved_at     DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_session_q (session_id, question_id),
  CONSTRAINT fk_answers_session FOREIGN KEY (session_id) REFERENCES exam_sessions(id) ON DELETE CASCADE,
  CONSTRAINT fk_answers_question FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE,
  INDEX idx_answers_question (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS refresh_tokens (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id     BIGINT NOT NULL,
  family_id   VARCHAR(36) NOT NULL,
  token_hash  CHAR(64) NOT NULL,
  expires_at  DATETIME(3) NOT NULL,
  revoked_at  DATETIME(3) NULL,
  created_at  DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_refresh_hash (token_hash),
  CONSTRAINT fk_refresh_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  INDEX idx_refresh_user (user_id),
  INDEX idx_refresh_family (family_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS audit_logs (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  actor_id    BIGINT NULL,
  action      VARCHAR(64) NOT NULL,
  target      VARCHAR(64) NULL,
  details     JSON NULL,
  client_ip   VARCHAR(45) NULL,
  created_at  DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  CONSTRAINT fk_audit_actor FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE SET NULL,
  INDEX idx_audit_actor (actor_id),
  INDEX idx_audit_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
