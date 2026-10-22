-- Theory marking: examiner marking guides on written questions and
-- examiner-confirmed marks (plus the last AI suggestion) on answers.
ALTER TABLE questions
  ADD COLUMN marking_guide JSON NULL AFTER marks,
  ADD COLUMN model_answer TEXT NULL AFTER marking_guide;
ALTER TABLE answers
  ADD COLUMN awarded_marks DECIMAL(5,2) NULL AFTER saved_at,
  ADD COLUMN marked_by BIGINT NULL AFTER awarded_marks,
  ADD COLUMN marked_at DATETIME(3) NULL AFTER marked_by,
  ADD COLUMN marker_comment TEXT NULL AFTER marked_at,
  ADD COLUMN suggestion JSON NULL AFTER marker_comment;
