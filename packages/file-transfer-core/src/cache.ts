import type { AccessGrant } from './types.js'

interface CacheEntry {
  grant: AccessGrant
  sizeBytes: number
}

export class AccessGrantCache {
  private readonly entries = new Map<string, CacheEntry>()

  constructor(
    private readonly maxEntries: number,
    private readonly maxFileBytes: number,
    private readonly expirySkewMs: number,
    private readonly now: () => number,
  ) {}

  get(key: string): AccessGrant | undefined {
    const entry = this.entries.get(key)
    if (!entry) return undefined
    const refreshAt = Date.parse(entry.grant.refreshAfter)
    const expiresAt = Date.parse(entry.grant.expiresAt)
    const validUntil = Number.isFinite(refreshAt) ? Math.min(refreshAt, expiresAt) : expiresAt
    if (!Number.isFinite(validUntil) || validUntil - this.expirySkewMs <= this.now()) {
      this.entries.delete(key)
      return undefined
    }
    this.entries.delete(key)
    this.entries.set(key, entry)
    return entry.grant
  }

  set(key: string, grant: AccessGrant, sizeBytes: number): void {
    if (this.maxEntries <= 0 || sizeBytes > this.maxFileBytes) return
    this.entries.delete(key)
    this.entries.set(key, { grant, sizeBytes })
    while (this.entries.size > this.maxEntries) {
      const oldest = this.entries.keys().next().value as string | undefined
      if (!oldest) break
      this.entries.delete(oldest)
    }
  }

  deleteMatching(predicate: (key: string) => boolean): void {
    for (const key of this.entries.keys()) {
      if (predicate(key)) this.entries.delete(key)
    }
  }

  clear(): void {
    this.entries.clear()
  }
}

