-- Register episode_info columns in _columns table so it appears in the dashboard
INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary, description)
VALUES
    ('episode_info', 'id', 'integer', 0, NULL, 1, 'Primary key'),
    ('episode_info', 'season', 'integer', 0, NULL, 0, 'Season number'),
    ('episode_info', 'episode_no', 'integer', 0, NULL, 0, 'Episode number within season'),
    ('episode_info', 'title', 'text', 0, NULL, 0, 'Episode title'),
    ('episode_info', 'air_date', 'text', 1, NULL, 0, 'Original air date'),
    ('episode_info', 'writers', 'text', 1, NULL, 0, 'Episode writers'),
    ('episode_info', 'director', 'text', 1, NULL, 0, 'Episode director'),
    ('episode_info', 'seid', 'text', 0, NULL, 0, 'Season/episode ID (e.g., S02E11)');
