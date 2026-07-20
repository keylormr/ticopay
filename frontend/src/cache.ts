import { useEffect, useReducer, useRef } from 'react'

// A tiny cache-with-selective-invalidation for read endpoints. It shows the
// last known data for a key instantly (so returning to a tab doesn't flash a
// "Cargando…" and refetch from scratch every time) and lets a mutation refresh
// exactly the lists it changed via invalidate(), replacing the old
// dashboard-wide "version" counter that refetched every section at once.
const cache = new Map<string, unknown>()
const inflight = new Map<string, Promise<void>>()
const subs = new Map<string, Set<() => void>>()

function emit(key: string) {
  subs.get(key)?.forEach((fn) => fn())
}

// load fetches a key at most once per miss: a cache hit or an in-flight request
// short-circuits, so concurrent subscribers and re-renders don't stampede.
function load<T>(key: string, fetcher: () => Promise<T>) {
  if (cache.has(key) || inflight.has(key)) return
  const p = fetcher()
    .then((data) => {
      cache.set(key, data)
    })
    .catch(() => {
      // Leave the key empty on failure so a later mount/invalidate retries.
    })
    .finally(() => {
      inflight.delete(key)
      emit(key)
    })
  inflight.set(key, p)
}

// invalidate drops the cached value for each key and notifies mounted readers,
// which then refetch. Call after a mutation that changes what those keys return.
export function invalidate(...keys: string[]) {
  for (const key of keys) {
    cache.delete(key)
    emit(key)
  }
}

export interface CachedResult<T> {
  data: T | undefined
  loading: boolean
}

// useCached returns the cached value for key (instantly on a hit) and fetches on
// a miss. It re-renders when the key's value changes or is invalidated.
export function useCached<T>(key: string, fetcher: () => Promise<T>): CachedResult<T> {
  const [, rerender] = useReducer((n: number) => n + 1, 0)
  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  useEffect(() => {
    const onChange = () => {
      load(key, fetcherRef.current)
      rerender()
    }
    let set = subs.get(key)
    if (!set) {
      set = new Set()
      subs.set(key, set)
    }
    set.add(onChange)
    load(key, fetcherRef.current)
    return () => {
      set!.delete(onChange)
      if (set!.size === 0) subs.delete(key)
    }
  }, [key])

  return {
    data: cache.get(key) as T | undefined,
    loading: !cache.has(key),
  }
}
