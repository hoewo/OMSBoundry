import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const source = fs.readFileSync(new URL('./oms_phase1.js', import.meta.url), 'utf8')
const match = source.match(/function partyVisibleForRole\(kind,id,clientIDs,contractorIDs\)\{[^}]+\}/)
assert.ok(match, 'party visibility rule must remain directly testable')
const partyVisibleForRole = Function(`return (${match[0]})`)()

test('unlinked external parties remain visible as pre-project clients', () => {
  assert.equal(partyVisibleForRole('client', 'unlinked', new Set(), new Set()), true)
  assert.equal(partyVisibleForRole('contractor', 'unlinked', new Set(), new Set()), false)
})

test('confirmed project roles remain separated without losing dual-role parties', () => {
  const clients = new Set(['client', 'both'])
  const contractors = new Set(['contractor', 'both'])

  assert.equal(partyVisibleForRole('client', 'client', clients, contractors), true)
  assert.equal(partyVisibleForRole('client', 'contractor', clients, contractors), false)
  assert.equal(partyVisibleForRole('contractor', 'client', clients, contractors), false)
  assert.equal(partyVisibleForRole('contractor', 'contractor', clients, contractors), true)
  assert.equal(partyVisibleForRole('client', 'both', clients, contractors), true)
  assert.equal(partyVisibleForRole('contractor', 'both', clients, contractors), true)
})

test('pre-project candidate roles are loaded from Project Workspace and saved before refresh', () => {
  assert.match(source, /for\(const candidate of state\.partyCandidates\)/)
  assert.match(source, /candidate\.role_key==='contractor'\)contractorIDs\.add\(candidate\.party_id\)/)
  assert.match(source, /api\('project-workspace','\/api\/v1\/project-party-candidates',\{method:'POST'/)
  assert.match(source, /await loadAll\(\);activatePage\('projects',roleKey==='client'\?'clients':'contractors'\)/)
  assert.match(source, /activePartiesForRole\('contractor'\)/)
})
