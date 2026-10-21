-- Per-exam proctoring switches (JSON of booleans). NULL / missing keys mean
-- "on", so existing exams keep full proctoring.
ALTER TABLE exams ADD COLUMN proctoring JSON NULL AFTER pin_code;
