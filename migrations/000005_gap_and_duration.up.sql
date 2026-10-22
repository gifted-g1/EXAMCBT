-- Fill-in-the-gap questions (accepted answers per blank) and a per-student
-- exam time limit in minutes (0 = until the exam window ends).
ALTER TABLE questions ADD COLUMN blanks JSON NULL AFTER correct_option;
ALTER TABLE exams ADD COLUMN duration_minutes INT NOT NULL DEFAULT 0 AFTER proctoring;
ALTER TABLE questions MODIFY COLUMN kind ENUM('MCQ','TEXT','GAP') NOT NULL DEFAULT 'MCQ';
