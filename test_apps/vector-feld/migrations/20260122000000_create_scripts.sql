-- Create scripts table for Seinfeld dialogue
CREATE TABLE scripts (
  id INTEGER PRIMARY KEY,
  character TEXT NOT NULL,
  dialogue TEXT NOT NULL,
  episode_no INTEGER,
  seid TEXT,
  season INTEGER,
  embedding TEXT
);

-- Indexes for common queries
CREATE INDEX idx_scripts_character ON scripts(character);
CREATE INDEX idx_scripts_season ON scripts(season);
CREATE INDEX idx_scripts_seid ON scripts(seid);

-- Register embedding column as vector(768) for Gemini text-embedding-004
INSERT INTO _columns (table_name, column_name, column_type)
VALUES ('scripts', 'embedding', 'vector(768)');
