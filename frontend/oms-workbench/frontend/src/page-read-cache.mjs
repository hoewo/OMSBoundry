export const DEFAULT_PAGE_READ_FRESH_MS = 30_000

// Page-memory only. It remembers freshness and in-flight reads, never tokens or
// business payloads. The owning adapter remains the sole holder of its current
// projection and every refresh still reaches the native application.
export class PageReadCache {
  constructor({freshMs = DEFAULT_PAGE_READ_FRESH_MS, now = () => Date.now()} = {}) {
    if (!Number.isFinite(freshMs) || freshMs < 0) throw new Error('invalid page read freshness')
    this.freshMs = freshMs
    this.now = now
    this.loadedAt = new Map()
    this.inflight = new Map()
  }

  isFresh(key, hasValue) {
    const loadedAt = this.loadedAt.get(key)
    return !!hasValue && Number.isFinite(loadedAt) && this.now() - loadedAt < this.freshMs
  }

  read(key, loader) {
    const existing = this.inflight.get(key)
    if (existing) return existing
    const task = Promise.resolve()
      .then(loader)
      .then(value => {
        this.loadedAt.set(key, this.now())
        return value
      })
      .finally(() => {
        if (this.inflight.get(key) === task) this.inflight.delete(key)
      })
    this.inflight.set(key, task)
    return task
  }

  markFresh(key) { this.loadedAt.set(key, this.now()) }
  markStale(key) { this.loadedAt.delete(key) }
  clear() { this.loadedAt.clear(); this.inflight.clear() }
}
