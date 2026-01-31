/**
 * Tables Store
 * Tables and data management state
 */

import { create } from 'zustand'
import { tablesApi, dataApi } from '@/lib/api-client'

interface Column {
  name: string
  type: string
  nullable: boolean
  primary_key: boolean
  default_value?: string
  description?: string
}

interface Filter {
  column: string
  operator: 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'like' | 'ilike' | 'is' | 'in'
  value: unknown
}

interface Sort {
  column: string
  direction: 'asc' | 'desc'
}

interface TablesState {
  // Data
  tables: string[]
  selectedTable: string | null
  schema: Column[] | null
  data: unknown[]
  total: number

  // UI state
  loading: boolean
  error: string | null

  // Query state
  filters: Filter[]
  sort: Sort | null
  pagination: {
    page: number
    pageSize: number
  }

  // Actions
  loadTables: () => Promise<void>
  selectTable: (name: string) => Promise<void>
  createTable: (data: {
    name: string
    columns: Array<{ name: string; type: string; nullable?: boolean }>
  }) => Promise<void>
  deleteTable: (name: string) => Promise<void>

  // Schema
  addColumn: (data: { name: string; type: string; nullable?: boolean }) => Promise<void>
  renameColumn: (oldName: string, newName: string) => Promise<void>
  deleteColumn: (name: string) => Promise<void>

  // Data
  loadData: () => Promise<void>
  updateCell: (rowIndex: number, column: string, value: unknown) => Promise<void>
  createRow: (row: Record<string, unknown>) => Promise<void>
  deleteRows: (rowIndices: number[]) => Promise<void>

  // Query
  setFilter: (index: number, filter: Filter) => void
  addFilter: (filter: Filter) => void
  removeFilter: (index: number) => void
  setSort: (sort: Sort | null) => void
  setPage: (page: number) => void
  setPageSize: (pageSize: number) => void

  // Clear
  clearError: () => void
  clearSelection: () => void
}

export const useTablesStore = create<TablesState>((set, get) => ({
  // Initial state
  tables: [],
  selectedTable: null,
  schema: null,
  data: [],
  total: 0,
  loading: false,
  error: null,
  filters: [],
  sort: null,
  pagination: {
    page: 1,
    pageSize: 50,
  },

  // Load all tables
  loadTables: async () => {
    set({ loading: true, error: null })
    try {
      const tables = await tablesApi.list()
      set({ tables, loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to load tables',
        loading: false,
      })
    }
  },

  // Select a table and load its schema and data
  selectTable: async (name: string) => {
    set({ loading: true, error: null, selectedTable: name })
    try {
      // Load schema
      const tableData = await tablesApi.get(name)
      const schema: Column[] = (tableData.columns as unknown[]).map((col: unknown) => {
        const c = col as Record<string, unknown>
        return {
          name: c.name as string,
          type: c.type as string,
          nullable: c.nullable as boolean,
          primary_key: c.primary_key as boolean,
          default_value: c.default_value as string | undefined,
          description: c.description as string | undefined,
        }
      })

      set({ schema, loading: false })
      await get().loadData()
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to load table',
        loading: false,
        selectedTable: null,
      })
    }
  },

  // Create a new table
  createTable: async (data) => {
    set({ loading: true, error: null })
    try {
      await tablesApi.create(data)
      await get().loadTables()
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to create table',
        loading: false,
      })
      throw error
    }
  },

  // Delete a table
  deleteTable: async (name: string) => {
    set({ loading: true, error: null })
    try {
      await tablesApi.delete(name)
      await get().loadTables()
      if (get().selectedTable === name) {
        set({ selectedTable: null, schema: null, data: [], filters: [], sort: null })
      }
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to delete table',
        loading: false,
      })
      throw error
    }
  },

  // Add a column to the selected table
  addColumn: async (data) => {
    const tableName = get().selectedTable
    if (!tableName) throw new Error('No table selected')

    set({ loading: true, error: null })
    try {
      await tablesApi.addColumn(tableName, data)
      await get().selectTable(tableName)
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to add column',
        loading: false,
      })
      throw error
    }
  },

  // Rename a column
  renameColumn: async (oldName: string, newName: string) => {
    const tableName = get().selectedTable
    if (!tableName) throw new Error('No table selected')

    set({ loading: true, error: null })
    try {
      await tablesApi.renameColumn(tableName, oldName, newName)
      await get().selectTable(tableName)
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to rename column',
        loading: false,
      })
      throw error
    }
  },

  // Delete a column
  deleteColumn: async (name: string) => {
    const tableName = get().selectedTable
    if (!tableName) throw new Error('No table selected')

    set({ loading: true, error: null })
    try {
      await tablesApi.deleteColumn(tableName, name)
      await get().selectTable(tableName)
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to delete column',
        loading: false,
      })
      throw error
    }
  },

  // Load data for the selected table
  loadData: async () => {
    const tableName = get().selectedTable
    if (!tableName) return

    set({ loading: true, error: null })
    try {
      const { filters, sort, pagination } = get()

      // Build query params
      const params: Record<string, string> = {}
      if (pagination.pageSize > 0) {
        params.limit = pagination.pageSize.toString()
      }
      params.offset = ((pagination.page - 1) * pagination.pageSize).toString()

      if (sort) {
        params.order = `${sort.direction === 'asc' ? '' : '-'}${sort.column}`
      }

      // Apply filters
      const filtersCopy = [...filters]
      for (const filter of filtersCopy) {
        // Apply filters via query params
        const operator = filter.operator
        const value = typeof filter.value === 'string' ? filter.value : JSON.stringify(filter.value)
        params[`${filter.column}.${operator}`] = value as string
      }

      const result = await dataApi.list(tableName, params)
      set({ data: result.data, total: result.total, loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to load data',
        loading: false,
      })
    }
  },

  // Update a cell value
  updateCell: async (rowIndex: number, column: string, value: unknown) => {
    const tableName = get().selectedTable
    if (!tableName) return

    const rowData = get().data[rowIndex] as Record<string, unknown> | undefined
    if (!rowData) return

    set({ loading: true, error: null })
    try {
      await dataApi.update(tableName, { ...rowData, [column]: value })
      await get().loadData()
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to update cell',
        loading: false,
      })
      throw error
    }
  },

  // Create a new row
  createRow: async (row: Record<string, unknown>) => {
    const tableName = get().selectedTable
    if (!tableName) throw new Error('No table selected')

    set({ loading: true, error: null })
    try {
      await dataApi.create(tableName, row)
      await get().loadData()
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to create row',
        loading: false,
      })
      throw error
    }
  },

  // Delete rows
  deleteRows: async (rowIndices: number[]) => {
    const tableName = get().selectedTable
    if (!tableName) throw new Error('No table selected')

    const data = get().data
    const schema = get().schema
    if (!schema || schema.length === 0) throw new Error('No schema loaded')

    // Get primary key column
    const pkColumn = schema.find((col) => col.primary_key)
    if (!pkColumn) throw new Error('No primary key column found')

    set({ loading: true, error: null })
    try {
      for (const index of rowIndices) {
        const row = data[index] as Record<string, unknown>
        const pkValue = row[pkColumn.name]
        await dataApi.delete(tableName, { [pkColumn.name]: pkValue })
      }
      await get().loadData()
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to delete rows',
        loading: false,
      })
      throw error
    }
  },

  // Set a specific filter
  setFilter: (index: number, filter: Filter) => {
    set((state) => {
      const filters = [...state.filters]
      filters[index] = filter
      return { filters, pagination: { ...state.pagination, page: 1 } }
    })
    get().loadData()
  },

  // Add a new filter
  addFilter: (filter: Filter) => {
    set((state) => ({
      filters: [...state.filters, filter],
      pagination: { ...state.pagination, page: 1 },
    }))
    get().loadData()
  },

  // Remove a filter
  removeFilter: (index: number) => {
    set((state) => {
      const filters = state.filters.filter((_, i) => i !== index)
      return { filters, pagination: { ...state.pagination, page: 1 } }
    })
    get().loadData()
  },

  // Set sort
  setSort: (sort: Sort | null) => {
    set({ sort })
    get().loadData()
  },

  // Set page
  setPage: (page: number) => {
    set((state) => ({
      pagination: { ...state.pagination, page },
    }))
    get().loadData()
  },

  // Set page size
  setPageSize: (pageSize: number) => {
    set(() => ({
      pagination: { page: 1, pageSize },
    }))
    get().loadData()
  },

  // Clear error
  clearError: () => set({ error: null }),

  // Clear selection
  clearSelection: () => set({
    selectedTable: null,
    schema: null,
    data: [],
    filters: [],
    sort: null,
    pagination: { page: 1, pageSize: 50 },
  }),
}))

