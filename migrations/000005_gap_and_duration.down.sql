DELETE FROM questions WHERE kind = 'GAP';
ALTER TABLE questions MODIFY COLUMN kind ENUM('MCQ','TEXT') NOT NULL DEFAULT 'MCQ';
ALTER TABLE exams DROP COLUMN duration_minutes;
ALTER TABLE questions DROP COLUMN blanks;
