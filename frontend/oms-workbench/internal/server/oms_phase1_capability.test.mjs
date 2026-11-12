import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'

const source = fs.readFileSync(new URL('./oms_phase1_capability.js', import.meta.url), 'utf8')

function loadFactory() {
  const context = {}
  context.globalThis = context
  vm.runInNewContext(source, context)
  return context.OMSPhase1CapabilityCoordinator
}

function lease(id, audience, seconds, clock) {
  return {application_id: id, audience, expires_at: new Date(clock + seconds * 1000).toISOString(), expires_in_seconds: seconds}
}

test('capability coordinator renews each audience before expiry and coalesces concurrent exchanges', async () => {
  let clock = 0
  let exchanges = 0
  const timers = new Map()
  let nextTimer = 1
  const coordinator = loadFactory().create({
    now: () => clock,
    setTimer: (callback, delay) => { const id = nextTimer++; timers.set(id, {callback, delay}); return id },
    clearTimer: id => timers.delete(id),
    exchange: async (id, audience) => { exchanges++; await Promise.resolve(); return lease(id, audience, 300, clock) }
  })

  const [first, second] = await Promise.all([
    coordinator.ensure('contract-ledger', 'contract-ledger'),
    coordinator.ensure('contract-ledger', 'contract-ledger')
  ])
  assert.equal(exchanges, 1)
  assert.equal(first, second)
  assert.equal(first.renewAt, 240000)
  assert.equal([...timers.values()][0].delay, 240000)

  clock = 240000
  await coordinator.recheck()
  assert.equal(exchanges, 2)

  await Promise.all([
    coordinator.requireFresh('contract-ledger', 'contract-ledger'),
    coordinator.requireFresh('contract-ledger', 'contract-ledger')
  ])
  assert.equal(exchanges, 3)
})

test('transient renewal failure keeps an unexpired read lease but blocks strict write renewal', async () => {
  let clock = 0
  let fail = false
  const coordinator = loadFactory().create({
    now: () => clock,
    setTimer: () => 1,
    clearTimer: () => {},
    exchange: async (id, audience) => {
      if (fail) { const error = new Error('IAM unavailable'); error.status = 502; throw error }
      return lease(id, audience, 300, clock)
    }
  })

  const original = await coordinator.ensure('money-flow-ledger', 'money-flow-ledger')
  clock = original.renewAt
  fail = true
  assert.equal(await coordinator.ensure('money-flow-ledger', 'money-flow-ledger'), original)
  await assert.rejects(coordinator.requireFresh('money-flow-ledger', 'money-flow-ledger'), /IAM unavailable/)
  assert.equal(coordinator.snapshot('money-flow-ledger'), original)

  clock = original.expiresAt + 1
  await coordinator.recheck()
  assert.equal(coordinator.snapshot('money-flow-ledger'), null)
})

test('definitive IAM denial clears the local lease and lease metadata contains no credential storage path', async () => {
  let denied = false
  let cleared = 0
  const coordinator = loadFactory().create({
    setTimer: () => 1,
    clearTimer: () => {},
    exchange: async (id, audience) => {
      if (denied) { const error = new Error('denied'); error.status = 403; throw error }
      return lease(id, audience, 300, Date.now())
    },
    onClear: () => { cleared++ }
  })

  await coordinator.ensure('billing-invoice', 'billing-invoice')
  denied = true
  await assert.rejects(coordinator.requireFresh('billing-invoice', 'billing-invoice'), /denied/)
  assert.equal(coordinator.snapshot('billing-invoice'), null)
  assert.equal(cleared, 1)
  assert.doesNotMatch(source, /localStorage|sessionStorage|document\.cookie|authorization\s*:/i)
})
