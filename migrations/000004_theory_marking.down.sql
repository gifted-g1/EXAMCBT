ALTER TABLE answers DROP COLUMN suggestion, DROP COLUMN marker_comment, DROP COLUMN marked_at, DROP COLUMN marked_by, DROP COLUMN awarded_marks;
ALTER TABLE questions DROP COLUMN model_answer, DROP COLUMN marking_guide;
