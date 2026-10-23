-- Per-student random question draw: how many questions each student gets
-- (0 = all), and which questions (in order) each session was given.
ALTER TABLE exams ADD COLUMN questions_per_student INT NOT NULL DEFAULT 0 AFTER duration_minutes;
ALTER TABLE exam_sessions ADD COLUMN question_ids JSON NULL AFTER status;
