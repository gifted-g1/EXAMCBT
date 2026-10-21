-- Optional academic department on the roster (e.g. "Information Technology").
ALTER TABLE students ADD COLUMN department VARCHAR(120) NULL AFTER full_name;
