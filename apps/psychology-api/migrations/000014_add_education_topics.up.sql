ALTER TABLE courses ADD COLUMN topic text NOT NULL DEFAULT '' CHECK (char_length(topic) <= 80);
ALTER TABLE workshops ADD COLUMN topic text NOT NULL DEFAULT '' CHECK (char_length(topic) <= 80);
