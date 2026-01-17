#!/usr/bin/env tsx
/**
 * Setup test database with sample data for E2E tests
 *
 * Run: npx tsx scripts/setup-test-db.ts
 */

import { execSync } from 'child_process'
import * as fs from 'fs'
import * as path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const DB_PATH = process.env.SBLITE_DB_PATH || path.join(__dirname, '../../test.db')
const SBLITE_BIN = path.join(__dirname, '../../sblite')

console.log('🔧 Setting up test database...')
console.log(`   Database path: ${DB_PATH}`)

// Remove existing test database
if (fs.existsSync(DB_PATH)) {
  console.log('   Removing existing database...')
  fs.unlinkSync(DB_PATH)
  // Also remove WAL files
  if (fs.existsSync(DB_PATH + '-wal')) fs.unlinkSync(DB_PATH + '-wal')
  if (fs.existsSync(DB_PATH + '-shm')) fs.unlinkSync(DB_PATH + '-shm')
}

// Initialize fresh database
console.log('   Initializing database...')
try {
  execSync(`cd ${path.dirname(DB_PATH)} && go run ../main.go init --db ${DB_PATH}`, {
    cwd: path.join(__dirname, '../..'),
    stdio: 'inherit',
  })
} catch (e) {
  // If sblite binary doesn't exist, build it first
  console.log('   Building sblite...')
  execSync('go build -o sblite .', {
    cwd: path.join(__dirname, '../..'),
    stdio: 'inherit',
  })
  execSync(`./sblite init --db ${DB_PATH}`, {
    cwd: path.join(__dirname, '../..'),
    stdio: 'inherit',
  })
}

// Create test tables using SQLite directly
console.log('   Creating test tables...')

const createTablesSQL = `
-- Characters table (Star Wars theme from Supabase docs)
CREATE TABLE IF NOT EXISTS characters (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  homeworld TEXT,
  is_jedi INTEGER  -- Boolean: 0=false, 1=true, NULL=unknown
);

-- Countries table
CREATE TABLE IF NOT EXISTS countries (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  code TEXT
);

-- Orchestral sections table
CREATE TABLE IF NOT EXISTS orchestral_sections (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);

-- Instruments table
CREATE TABLE IF NOT EXISTS instruments (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  section_id INTEGER REFERENCES orchestral_sections(id)
);

-- Cities table
CREATE TABLE IF NOT EXISTS cities (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  country_id INTEGER REFERENCES countries(id),
  population INTEGER
);

-- Users table (with JSON support)
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  address TEXT -- JSON stored as TEXT
);

-- Issues table (with array support via JSON)
CREATE TABLE IF NOT EXISTS issues (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL,
  tags TEXT -- JSON array stored as TEXT
);

-- Quotes table (for text search)
CREATE TABLE IF NOT EXISTS quotes (
  id INTEGER PRIMARY KEY,
  catchphrase TEXT NOT NULL
);

-- Classes table (for containedBy tests)
CREATE TABLE IF NOT EXISTS classes (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  days TEXT -- JSON array
);

-- Reservations table (for range tests)
CREATE TABLE IF NOT EXISTS reservations (
  id INTEGER PRIMARY KEY,
  room TEXT NOT NULL,
  during TEXT -- Range stored as TEXT
);

-- Messages table (for self-join)
CREATE TABLE IF NOT EXISTS messages (
  id INTEGER PRIMARY KEY,
  content TEXT NOT NULL,
  sender_id INTEGER,
  receiver_id INTEGER
);

-- Teams table
CREATE TABLE IF NOT EXISTS teams (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);

-- User-Teams junction table
CREATE TABLE IF NOT EXISTS user_teams (
  user_id INTEGER,
  team_id INTEGER,
  PRIMARY KEY (user_id, team_id)
);

-- Texts table (for full-text search)
CREATE TABLE IF NOT EXISTS texts (
  id INTEGER PRIMARY KEY,
  content TEXT NOT NULL
);

-- RLS test table (for Row Level Security tests)
CREATE TABLE IF NOT EXISTS rls_test (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT,
  data TEXT
);

-- Insert test data
INSERT OR REPLACE INTO characters (id, name, homeworld, is_jedi) VALUES
  (1, 'Luke', 'Tatooine', 1),      -- true
  (2, 'Leia', 'Alderaan', 0),      -- false (not trained as Jedi in original trilogy)
  (3, 'Han', 'Corellia', 0),       -- false
  (4, 'Yoda', 'Dagobah', 1),       -- true
  (5, 'Chewbacca', 'Kashyyyk', 0); -- false

INSERT OR REPLACE INTO countries (id, name, code) VALUES
  (1, 'United States', 'US'),
  (2, 'Canada', 'CA'),
  (3, 'Mexico', 'MX');

INSERT OR REPLACE INTO orchestral_sections (id, name) VALUES
  (1, 'strings'),
  (2, 'woodwinds'),
  (3, 'percussion');

INSERT OR REPLACE INTO instruments (id, name, section_id) VALUES
  (1, 'violin', 1),
  (2, 'viola', 1),
  (3, 'flute', 2),
  (4, 'clarinet', 2),
  (5, 'piano', 3);

INSERT OR REPLACE INTO cities (id, name, country_id, population) VALUES
  (1, 'New York', 1, 8336817),
  (2, 'Los Angeles', 1, 3979576),
  (3, 'Toronto', 2, 2731571),
  (4, 'Vancouver', 2, 631486),
  (5, 'Smalltown', 1, 5000);

INSERT OR REPLACE INTO users (id, name, address) VALUES
  (1, 'John Doe', '{"street":"123 Main St","city":"New York","postcode":10001}'),
  (2, 'Jane Smith', '{"street":"456 Oak Ave","city":"Beverly Hills","postcode":90210}');

INSERT OR REPLACE INTO issues (id, title, tags) VALUES
  (1, 'Bug: Login fails', '["is:open","priority:high"]'),
  (2, 'Feature: Dark mode', '["is:open","priority:low"]'),
  (3, 'Bug: Fixed crash', '["is:closed","severity:high"]');

INSERT OR REPLACE INTO quotes (id, catchphrase) VALUES
  (1, 'The quick brown fox jumps over the lazy dog'),
  (2, 'The fat cat sat on the mat'),
  (3, 'A rolling stone gathers no moss');

INSERT OR REPLACE INTO classes (id, name, days) VALUES
  (1, 'Morning Yoga', '["monday","wednesday","friday"]'),
  (2, 'Evening Spin', '["tuesday","thursday"]'),
  (3, 'Weekend Run', '["saturday","sunday"]');

INSERT OR REPLACE INTO reservations (id, room, during) VALUES
  (1, 'A', '[2000-01-01 09:00, 2000-01-01 10:00)'),
  (2, 'B', '[2000-01-01 12:00, 2000-01-01 14:00)'),
  (3, 'A', '[2000-01-02 08:00, 2000-01-02 09:00)');

INSERT OR REPLACE INTO messages (id, content, sender_id, receiver_id) VALUES
  (1, 'Hello!', 1, 2),
  (2, 'Hi there!', 2, 1);

INSERT OR REPLACE INTO teams (id, name) VALUES
  (1, 'Team Alpha'),
  (2, 'Team Beta');

INSERT OR REPLACE INTO user_teams (user_id, team_id) VALUES
  (1, 1),
  (1, 2),
  (2, 1);

INSERT OR REPLACE INTO texts (id, content) VALUES
  (1, 'Green eggs and ham are delicious'),
  (2, 'I do not like them Sam I am'),
  (3, 'Would you eat them in a box');
`

// Write SQL to temp file and execute
const sqlFile = path.join(__dirname, 'temp-setup.sql')
fs.writeFileSync(sqlFile, createTablesSQL)

try {
  execSync(`sqlite3 "${DB_PATH}" < "${sqlFile}"`, { stdio: 'inherit' })
  console.log('✅ Test database setup complete!')
} catch (e) {
  console.error('❌ Failed to setup test database:', e)
  process.exit(1)
} finally {
  fs.unlinkSync(sqlFile)
}

// Add RLS policy for rls_test table
console.log('   Adding RLS policies...')
try {
  execSync(`./sblite policy add --table rls_test --name user_isolation --using "user_id = auth.uid()" --check "user_id = auth.uid()" --db ${DB_PATH}`, {
    cwd: path.join(__dirname, '../..'),
    stdio: 'pipe',
  })
} catch (e) {
  // Policy might already exist or command might fail
  console.log('   Note: RLS policy may already exist or could not be added')
}

console.log(`
Test tables created:
   - characters (5 rows)
   - countries (3 rows)
   - orchestral_sections (3 rows)
   - instruments (5 rows)
   - cities (5 rows)
   - users (2 rows)
   - issues (3 rows)
   - quotes (3 rows)
   - classes (3 rows)
   - reservations (3 rows)
   - messages (2 rows)
   - teams (2 rows)
   - user_teams (3 rows)
   - texts (3 rows)
   - rls_test (0 rows, with RLS policy)

Ready to run tests!
`)
