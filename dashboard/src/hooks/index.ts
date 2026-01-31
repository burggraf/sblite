/**
 * Custom Hooks
 * Reusable hooks for common functionality
 */

import { useEffect, useCallback, useRef, useState } from 'react'
import { toast } from 'sonner'

/**
 * Hook to get a value from localStorage
 */
export function useLocalStorage<T>(key: string, initialValue: T) {
  const [storedValue, setStoredValue] = useState<T>(() => {
    if (typeof window === 'undefined') return initialValue

    try {
      const item = window.localStorage.getItem(key)
      return item ? JSON.parse(item) : initialValue
    } catch {
      return initialValue
    }
  })

  const setValue = useCallback(
    (value: T | ((val: T) => T)) => {
      try {
        const valueToStore = value instanceof Function ? value(storedValue) : value
        setStoredValue(valueToStore)
        if (typeof window !== 'undefined') {
          window.localStorage.setItem(key, JSON.stringify(valueToStore))
        }
      } catch (error) {
        console.error('Error saving to localStorage:', error)
      }
    },
    [key, storedValue]
  )

  return [storedValue, setValue] as const
}

/**
 * Hook to debounce a value
 */
export function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value)

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedValue(value)
    }, delay)

    return () => {
      clearTimeout(handler)
    }
  }, [value, delay])

  return debouncedValue
}

/**
 * Hook to get previous value
 */
export function usePrevious<T>(value: T): T | undefined {
  const ref = useRef<T>(undefined)

  useEffect(() => {
    ref.current = value
  }, [value])

  return ref.current
}

/**
 * Hook to detect click outside
 */
export function useClickOutside<T extends HTMLElement>(
  ref: React.RefObject<T>,
  handler: (event: MouseEvent | TouchEvent) => void
) {
  useEffect(() => {
    const listener = (event: MouseEvent | TouchEvent) => {
      if (!ref.current || ref.current.contains(event.target as Node)) {
        return
      }
      handler(event)
    }

    document.addEventListener('mousedown', listener)
    document.addEventListener('touchstart', listener)

    return () => {
      document.removeEventListener('mousedown', listener)
      document.removeEventListener('touchstart', listener)
    }
  }, [ref, handler])
}

/**
 * Hook to copy text to clipboard
 */
export function useClipboard() {
  const [isCopied, setIsCopied] = useState(false)

  const copy = useCallback(async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setIsCopied(true)
      setTimeout(() => setIsCopied(false), 2000)
    } catch (error) {
      console.error('Failed to copy:', error)
    }
  }, [])

  return { isCopied, copy }
}

/**
 * Hook to format file size
 */
export function useFileSize() {
  const format = useCallback((bytes: number): string => {
    if (bytes === 0) return '0 Bytes'

    const k = 1024
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))

    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
  }, [])

  return format
}

/**
 * Hook to format date/time
 */
export function useDateTime() {
  const format = useCallback((date: string | Date, format: 'relative' | 'absolute' = 'relative'): string => {
    const d = typeof date === 'string' ? new Date(date) : date

    if (format === 'absolute') {
      return d.toLocaleString()
    }

    // Relative format
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    const seconds = Math.floor(diff / 1000)
    const minutes = Math.floor(seconds / 60)
    const hours = Math.floor(minutes / 60)
    const days = Math.floor(hours / 24)

    if (seconds < 60) return 'just now'
    if (minutes < 60) return `${minutes}m ago`
    if (hours < 24) return `${hours}h ago`
    if (days < 7) return `${days}d ago`

    return d.toLocaleDateString()
  }, [])

  return format
}

/**
 * Hook to show toast notification using sonner
 */
export function useToast() {
  const success = useCallback((title: string, message?: string) => {
    if (message) {
      toast.success(title, { description: message })
    } else {
      toast.success(title)
    }
  }, [])

  const error = useCallback((title: string, message?: string) => {
    if (message) {
      toast.error(title, { description: message })
    } else {
      toast.error(title)
    }
  }, [])

  const warning = useCallback((title: string, message?: string) => {
    if (message) {
      toast.warning(title, { description: message })
    } else {
      toast.warning(title)
    }
  }, [])

  const info = useCallback((title: string, message?: string) => {
    if (message) {
      toast.info(title, { description: message })
    } else {
      toast.info(title)
    }
  }, [])

  return { success, error, warning, info }
}

/**
 * Hook to apply theme to document
 */
export function useThemeEffect() {
  useEffect(() => {
    if (typeof document !== 'undefined') {
      const root = document.documentElement
      // Default to dark mode
      root.classList.add('dark')
    }
  }, [])
}

/**
 * Hook to handle async operations with loading and error states
 */
export function useAsyncOperation<T = unknown>() {
  const [loading, setLoading] = useState<boolean>(false)
  const [error, setError] = useState<string | null>(null)
  const [data, setData] = useState<T | null>(null)

  const execute = useCallback(async (operation: () => Promise<T>) => {
    setLoading(true)
    setError(null)
    try {
      const result = await operation()
      setData(result)
      return result
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Operation failed'
      setError(errorMessage)
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const reset = useCallback(() => {
    setLoading(false)
    setError(null)
    setData(null)
  }, [])

  return { loading, error, data, execute, reset }
}

/**
 * Hook to get window size
 */
export function useWindowSize() {
  const [windowSize, setWindowSize] = useState({
    width: typeof window !== 'undefined' ? window.innerWidth : 0,
    height: typeof window !== 'undefined' ? window.innerHeight : 0,
  })

  useEffect(() => {
    if (typeof window === 'undefined') return

    const handleResize = () => {
      setWindowSize({
        width: window.innerWidth,
        height: window.innerHeight,
      })
    }

    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  return windowSize
}

/**
 * Hook to get media query match
 */
export function useMediaQuery(query: string) {
  const [matches, setMatches] = useState(() => {
    if (typeof window === 'undefined') return false
    return window.matchMedia(query).matches
  })

  useEffect(() => {
    if (typeof window === 'undefined') return

    const mediaQuery = window.matchMedia(query)
    const handler = () => setMatches(mediaQuery.matches)

    mediaQuery.addEventListener('change', handler)
    return () => mediaQuery.removeEventListener('change', handler)
  }, [query])

  return matches
}

/**
 * Hook to use keyboard shortcut
 */
export function useKeyboard(
  key: string,
  handler: (event: KeyboardEvent) => void,
  options?: { ctrl?: boolean; alt?: boolean; shift?: boolean }
) {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const keyMatches = event.key.toLowerCase() === key.toLowerCase()

      const ctrlMatches = options?.ctrl ? event.ctrlKey || event.metaKey : true
      const altMatches = options?.alt ? event.altKey : true
      const shiftMatches = options?.shift ? event.shiftKey : true

      if (keyMatches && ctrlMatches && altMatches && shiftMatches) {
        event.preventDefault()
        handler(event)
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [key, handler, options])
}
