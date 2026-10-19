import assert from 'node:assert/strict'
import test from 'node:test'
import {PageReadCache} from './page-read-cache.mjs'

test('a successful page read stays fresh only for the bounded page-memory window', async () => {
  let now = 1_000, reads = 0
  const cache = new PageReadCache({freshMs: 30_000, now: () => now})
  assert.equal(cache.isFresh('project-a:payments', false), false)
  await cache.read('project-a:payments', async () => { reads += 1 })
  assert.equal(reads, 1)
  assert.equal(cache.isFresh('project-a:payments', true), true)
  now += 29_999
  assert.equal(cache.isFresh('project-a:payments', true), true)
  now += 1
  assert.equal(cache.isFresh('project-a:payments', true), false)
})

test('rapid repeated tab selections share one native read', async () => {
  let release, reads = 0
  const pending = new Promise(resolve => { release = resolve })
  const cache = new PageReadCache()
  const loader = async () => { reads += 1; await pending; return 'ready' }
  const first = cache.read('project-a:contracts', loader)
  const second = cache.read('project-a:contracts', loader)
  assert.equal(first, second)
  assert.equal(reads, 0)
  await Promise.resolve()
  assert.equal(reads, 1)
  release()
  assert.equal(await first, 'ready')
  assert.equal(await second, 'ready')
})

test('failed reads are not marked fresh and explicit invalidation is isolated by key', async () => {
  let now = 50
  const cache = new PageReadCache({now: () => now})
  await assert.rejects(cache.read('project-a:costs', async () => { throw new Error('offline') }), /offline/)
  assert.equal(cache.isFresh('project-a:costs', true), false)
  await cache.read('project-a:costs', async () => {})
  await cache.read('project-a:bonus', async () => {})
  cache.markStale('project-a:costs')
  assert.equal(cache.isFresh('project-a:costs', true), false)
  assert.equal(cache.isFresh('project-a:bonus', true), true)
  now += 30_000
  assert.equal(cache.isFresh('project-a:bonus', true), false)
})

test('an owner-confirmed reread refreshes page freshness without another tab read', () => {
  let now = 1_000
  const cache = new PageReadCache({freshMs: 30_000, now: () => now})
  cache.markFresh('project-a:commission')
  now += 29_999
  assert.equal(cache.isFresh('project-a:commission', true), true)
  now += 1
  assert.equal(cache.isFresh('project-a:commission', true), false)
})
