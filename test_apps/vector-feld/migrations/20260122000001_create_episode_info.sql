-- Create episode_info table
CREATE TABLE IF NOT EXISTS episode_info (
    id INTEGER PRIMARY KEY,
    season INTEGER NOT NULL,
    episode_no INTEGER NOT NULL,
    title TEXT NOT NULL,
    air_date TEXT,
    writers TEXT,
    director TEXT,
    seid TEXT NOT NULL
);

-- Index on seid for fast lookups when displaying episode info for search results
CREATE INDEX IF NOT EXISTS idx_episode_info_seid ON episode_info(seid);

-- Also add index on scripts.seid if not exists for faster joins
CREATE INDEX IF NOT EXISTS idx_scripts_seid ON scripts(seid);
