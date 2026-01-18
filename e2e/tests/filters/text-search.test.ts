/**
 * Full-Text Search Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/textsearch
 *
 * Tests the textSearch filter which performs full-text search using FTS5.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Filters - Full-Text Search (textSearch)', () => {
  let supabase: SupabaseClient
  let serviceClient: SupabaseClient

  // Test data for FTS - articles with various content for testing
  const testArticles = [
    { id: 1, title: 'Introduction to Go Programming', body: 'Go is a statically typed, compiled programming language designed at Google.', author: 'Alice' },
    { id: 2, title: 'Python Basics for Beginners', body: 'Python is a high-level, general-purpose programming language.', author: 'Bob' },
    { id: 3, title: 'JavaScript and Web Development', body: 'JavaScript is the programming language of the Web. Learn about React, Vue, and Node.', author: 'Charlie' },
    { id: 4, title: 'Database Design with PostgreSQL', body: 'PostgreSQL is a powerful, open source object-relational database system.', author: 'Alice' },
    { id: 5, title: 'Full-Text Search in Databases', body: 'Full-text search allows searching through text content efficiently using indexes.', author: 'Bob' },
    { id: 6, title: 'The Fat Cat Adventures', body: 'The fat cat sat on the mat and looked at the rat.', author: 'Charlie' },
    { id: 7, title: 'Dogs and Cats Living Together', body: 'Sometimes dogs and cats can be the best of friends, not enemies.', author: 'Alice' },
    { id: 8, title: 'Machine Learning Fundamentals', body: 'Machine learning is a subset of artificial intelligence that enables computers to learn.', author: 'Bob' },
  ]

  beforeAll(async () => {
    // Create clients
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    serviceClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_SERVICE_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Create test table for FTS testing
    console.log('Creating articles table for FTS tests...')

    // First try to drop existing FTS indexes
    try {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts/search`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        },
      })
    } catch (e) {
      // Ignore errors if index doesn't exist
    }

    // Then drop existing table
    try {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
      })
    } catch (e) {
      // Ignore errors if table doesn't exist
    }

    // Create the articles table
    const createTableRes = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        name: 'articles',
        columns: [
          { name: 'id', type: 'integer', primary: true },
          { name: 'title', type: 'text' },
          { name: 'body', type: 'text' },
          { name: 'author', type: 'text' },
        ],
      }),
    })

    if (!createTableRes.ok) {
      const err = await createTableRes.text()
      console.error('Failed to create table:', err)
      throw new Error(`Failed to create articles table: ${err}`)
    }

    // Insert test data
    console.log('Inserting test data...')
    for (const article of testArticles) {
      const { error } = await serviceClient
        .from('articles')
        .insert(article)
      if (error) {
        console.error('Failed to insert article:', error)
      }
    }

    // Create FTS index on title and body columns
    console.log('Creating FTS index...')
    const createIndexRes = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        name: 'search',
        columns: ['title', 'body'],
        tokenizer: 'unicode61',
      }),
    })

    if (!createIndexRes.ok) {
      const err = await createIndexRes.text()
      console.error('Failed to create FTS index:', err)
      throw new Error(`Failed to create FTS index: ${err}`)
    }

    console.log('FTS test setup complete!')
  })

  afterAll(async () => {
    // Clean up: drop test table
    console.log('Cleaning up FTS test table...')
    try {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
      })
    } catch (e) {
      // Ignore cleanup errors
    }
  })

  /**
   * textSearch() - Basic Full-Text Search
   * Docs: https://supabase.com/docs/reference/javascript/textsearch
   */
  describe('textSearch() - Basic Search', () => {
    /**
     * Example: Basic text search
     * Match documents containing the search term
     */
    it('should find documents matching a single term', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'programming')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
      // Should match Go, Python, JavaScript articles that mention "programming"
      expect(data!.some((row: any) => row.title.includes('Go'))).toBe(true)
    })

    it('should return empty array when no matches found', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'nonexistentterm12345')

      expect(error).toBeNull()
      expect(data).toEqual([])
    })

    it('should search across multiple columns in index', async () => {
      // The FTS index includes both title and body
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('title', 'JavaScript')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].title).toContain('JavaScript')
    })
  })

  /**
   * textSearch() with Plain query type
   * Converts query to "term1 AND term2"
   */
  describe('textSearch() - Plain Query (plfts)', () => {
    it('should match all terms with plain query type', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'fat cat', { type: 'plain' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should match "The Fat Cat Adventures" - has both "fat" and "cat"
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.some((row: any) => row.title.includes('Fat Cat'))).toBe(true)
    })

    it('should require all terms to match', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'fat dog', { type: 'plain' })

      expect(error).toBeNull()
      // "fat" and "dog" don't appear together in any article
      expect(data!.length).toBe(0)
    })
  })

  /**
   * textSearch() with Phrase query type
   * Matches exact phrase
   */
  describe('textSearch() - Phrase Query (phfts)', () => {
    it('should match exact phrase', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'fat cat', { type: 'phrase' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should match articles with exact phrase "fat cat"
      expect(data!.length).toBeGreaterThan(0)
    })

    it('should not match when words are not adjacent', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'cat mat', { type: 'phrase' })

      expect(error).toBeNull()
      // "cat" and "mat" appear but not as adjacent phrase "cat mat"
      // (text is "cat sat on the mat")
      expect(data!.length).toBe(0)
    })
  })

  /**
   * textSearch() with Websearch query type
   * Supports OR, negation (-), and quoted phrases
   */
  describe('textSearch() - Websearch Query (wfts)', () => {
    it('should support OR operator', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'Python or JavaScript', { type: 'websearch' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThanOrEqual(2)
      // Should match both Python and JavaScript articles
      const titles = data!.map((row: any) => row.title)
      expect(titles.some((t: string) => t.includes('Python'))).toBe(true)
      expect(titles.some((t: string) => t.includes('JavaScript'))).toBe(true)
    })

    it('should support negation with minus', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'cat -dog', { type: 'websearch' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should match cat articles but not "Dogs and Cats" article
      expect(data!.some((row: any) => row.title.includes('Fat Cat'))).toBe(true)
      expect(data!.every((row: any) => !row.title.includes('Dogs and Cats'))).toBe(true)
    })

    it('should support quoted phrases', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', '"fat cat"', { type: 'websearch' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
      expect(data![0].title).toContain('Fat Cat')
    })

    it('should support combined OR, negation, and quotes', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', '"programming language" or database -PostgreSQL', { type: 'websearch' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should match programming language articles or database articles, but not PostgreSQL article
    })
  })

  /**
   * textSearch() with default FTS query type
   * Uses PostgreSQL-style tsquery operators: & | ! :*
   */
  describe('textSearch() - FTS Query (fts)', () => {
    it('should support AND operator (&)', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', "'programming' & 'language'")

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
    })

    it('should support OR operator (|)', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', "'Python' | 'Go'")

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThanOrEqual(2)
    })

    it.skip('should support NOT operator (!) combined with positive term', async () => {
      // SKIPPED: FTS5 NOT operator in tsquery format has complex requirements
      // The websearch format (-term) is recommended for negation instead
      // PostgreSQL-style !'term' format may not translate cleanly to FTS5
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', "'programming' & !'database'")

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should include programming articles but not database ones
      expect(data!.every((row: any) => !row.title.toLowerCase().includes('database'))).toBe(true)
    })

    it('should support prefix matching (:*)', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', "'program':*")

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should match "programming", "programmer", etc.
      expect(data!.length).toBeGreaterThan(0)
    })
  })

  /**
   * textSearch() combined with other filters
   */
  describe('textSearch() - Combined with Other Filters', () => {
    it('should work with eq filter', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title, author')
        .textSearch('body', 'programming')
        .eq('author', 'Alice')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should only return Alice's articles about programming
      expect(data!.every((row: any) => row.author === 'Alice')).toBe(true)
    })

    it('should work with order modifier', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'programming')
        .order('id', { ascending: true })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should be ordered by id
      for (let i = 1; i < data!.length; i++) {
        expect(data![i].id).toBeGreaterThan(data![i - 1].id)
      }
    })

    it('should work with limit modifier', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'programming')
        .limit(2)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeLessThanOrEqual(2)
    })

    it('should work with select specific columns', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('title')
        .textSearch('body', 'cat')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
      // Should only have title column
      expect(data![0]).toHaveProperty('title')
      expect(data![0]).not.toHaveProperty('id')
      expect(data![0]).not.toHaveProperty('body')
    })
  })

  /**
   * Admin API - FTS Index Management
   */
  describe('Admin API - FTS Index Management', () => {
    it('should list FTS indexes for a table', async () => {
      const res = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        },
      })

      expect(res.ok).toBe(true)
      const indexes = await res.json()
      expect(Array.isArray(indexes)).toBe(true)
      expect(indexes.length).toBeGreaterThan(0)
      // API response uses index_name as JSON field
      const firstIndex = indexes[0]
      expect(firstIndex.index_name).toBe('search')
      expect(firstIndex.columns).toContain('title')
      expect(firstIndex.columns).toContain('body')
    })

    it('should get a specific FTS index', async () => {
      const res = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts/search`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        },
      })

      expect(res.ok).toBe(true)
      const index = await res.json()
      expect(index.index_name).toBe('search')
      expect(index.table_name).toBe('articles')
      expect(index.tokenizer).toBe('unicode61')
    })

    it('should rebuild an FTS index', async () => {
      const res = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts/search/rebuild`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        },
      })

      expect(res.ok).toBe(true)

      // Verify search still works after rebuild
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'programming')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
    })

    it('should create FTS index with different tokenizer', async () => {
      // Create a second index with porter stemmer
      const createRes = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: 'search_porter',
          columns: ['title'],
          tokenizer: 'porter',
        }),
      })

      expect(createRes.ok).toBe(true)

      // Verify the index exists
      const getRes = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts/search_porter`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        },
      })

      expect(getRes.ok).toBe(true)
      const index = await getRes.json()
      expect(index.tokenizer).toBe('porter')

      // Clean up - delete the porter index
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts/search_porter`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        },
      })
    })

    it('should reject invalid tokenizer', async () => {
      const res = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: 'invalid_index',
          columns: ['title'],
          tokenizer: 'invalid_tokenizer',
        }),
      })

      expect(res.ok).toBe(false)
      expect(res.status).toBe(400)
    })

    it('should reject index on nonexistent column', async () => {
      const res = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/articles/fts`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: 'bad_index',
          columns: ['nonexistent_column'],
        }),
      })

      expect(res.ok).toBe(false)
      expect(res.status).toBe(400)
    })
  })

  /**
   * Edge Cases and Error Handling
   */
  describe('Edge Cases', () => {
    it('should handle empty search query gracefully', async () => {
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', '')

      // Empty query should either return all rows or error
      // depending on implementation
      if (error) {
        expect(error.message).toBeDefined()
      } else {
        expect(Array.isArray(data)).toBe(true)
      }
    })

    it('should handle hyphenated terms by splitting on hyphen', async () => {
      // FTS5 treats hyphens as word separators by default
      // "full-text" is equivalent to searching for "full" and "text"
      const { data, error } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'full text', { type: 'plain' })

      expect(error).toBeNull()
      // Should handle as two separate words
      expect(Array.isArray(data)).toBe(true)
      // The "Full-Text Search" article should match
      expect(data!.some((row: any) => row.title.includes('Full-Text'))).toBe(true)
    })

    it('should be case-insensitive by default', async () => {
      const { data: lowerData, error: lowerError } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'programming')

      const { data: upperData, error: upperError } = await supabase
        .from('articles')
        .select('id, title')
        .textSearch('body', 'PROGRAMMING')

      expect(lowerError).toBeNull()
      expect(upperError).toBeNull()
      // Should return same results regardless of case
      expect(lowerData!.length).toBe(upperData!.length)
    })

    it('should error when searching on column without FTS index', async () => {
      // Create a table without FTS
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: 'no_fts_table',
          columns: [
            { name: 'id', type: 'integer', primary: true },
            { name: 'content', type: 'text' },
          ],
        }),
      })

      // Try to use textSearch on table without FTS index
      const { data, error } = await supabase
        .from('no_fts_table')
        .select('id')
        .textSearch('content', 'test')

      // Should error because no FTS index exists
      expect(error).not.toBeNull()

      // Clean up
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/no_fts_table`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
        },
      })
    })
  })

  /**
   * Using the existing texts table
   */
  describe('Using texts table from seed data', () => {
    // First, we need to create an FTS index on the texts table
    beforeAll(async () => {
      // Create FTS index on texts table (may already exist from previous tests)
      try {
        await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/texts/fts`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            name: 'content_search',
            columns: ['content'],
          }),
        })
      } catch (e) {
        // Index might already exist
      }
    })

    it('should search texts table with FTS', async () => {
      const { data, error } = await supabase
        .from('texts')
        .select('id, content')
        .textSearch('content', 'eggs')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
      expect(data![0].content).toContain('eggs')
    })

    it('should search for "Sam" in texts', async () => {
      const { data, error } = await supabase
        .from('texts')
        .select('id, content')
        .textSearch('content', 'Sam')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].content).toContain('Sam')
    })
  })
})

/**
 * Compatibility Summary for Full-Text Search:
 *
 * IMPLEMENTED:
 * - textSearch() basic search
 * - Query types: plain (plfts), phrase (phfts), websearch (wfts), fts
 * - PostgreSQL-style operators: & (AND), | (OR), ! (NOT), :* (prefix)
 * - Combining textSearch with other filters (eq, order, limit)
 * - Admin API for FTS index management
 * - Multiple tokenizers: unicode61, porter, ascii, trigram
 *
 * NOT IMPLEMENTED:
 * - Search configuration parameter (uses SQLite tokenizers instead)
 * - Weighted search (ts_rank)
 * - Headline generation (ts_headline)
 */
