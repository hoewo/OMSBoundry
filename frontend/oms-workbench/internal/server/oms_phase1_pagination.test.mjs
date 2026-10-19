import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';

const require = createRequire(import.meta.url);
const { loadOffsetCollection, loadCursorCollection } = require('./oms_phase1_pagination.js');

test('offset collection loads every owner page and preserves filters', async () => {
  const source = Array.from({ length: 205 }, (_, index) => ({ id: `offset-${index + 1}` }));
  const calls = [];
  const api = async (id, path) => {
    calls.push({ id, path });
    const url = new URL(path, 'https://owner.invalid');
    const offset = Number(url.searchParams.get('offset'));
    const limit = Number(url.searchParams.get('limit'));
    assert.equal(url.searchParams.get('status'), 'all');
    const items = source.slice(offset, offset + limit);
    return { items, total: source.length, offset, limit, has_more: offset + items.length < source.length };
  };

  const result = await loadOffsetCollection(api, 'project-workspace', '/api/v1/project-directory', { status: 'all' });

  assert.deepEqual(calls.map(call => new URL(call.path, 'https://owner.invalid').searchParams.get('offset')), ['0', '100', '200']);
  assert.equal(result.items.length, 205);
  assert.equal(result.total, 205);
  assert.equal(result.has_more, false);
  assert.equal(result.items.at(-1).id, 'offset-205');
});

test('cursor collection loads every owner page and preserves filters', async () => {
  const pages = new Map([
    ['', { items: Array.from({ length: 100 }, (_, index) => ({ id: `cursor-${index + 1}` })), next_cursor: 'page-2' }],
    ['page-2', { items: Array.from({ length: 100 }, (_, index) => ({ id: `cursor-${index + 101}` })), next_cursor: 'page-3' }],
    ['page-3', { items: [{ id: 'cursor-201' }], next_cursor: null }],
  ]);
  const cursors = [];
  const api = async (id, path) => {
    const url = new URL(path, 'https://owner.invalid');
    const cursor = url.searchParams.get('cursor') || '';
    cursors.push(cursor);
    assert.equal(id, 'billing-invoice');
    assert.equal(url.searchParams.get('direction'), 'outgoing');
    return pages.get(cursor);
  };

  const result = await loadCursorCollection(api, 'billing-invoice', '/api/v1/invoice-registrations', { direction: 'outgoing' });

  assert.deepEqual(cursors, ['', 'page-2', 'page-3']);
  assert.equal(result.items.length, 201);
  assert.equal(result.total, 201);
  assert.equal(result.next_cursor, null);
  assert.equal(result.items.at(-1).id, 'cursor-201');
});

test('cursor collection fails closed when an owner repeats its cursor', async () => {
  let call = 0;
  const api = async () => {
    call++;
    return { items: [{ id: `item-${call}` }], next_cursor: 'same-cursor' };
  };

  await assert.rejects(
    loadCursorCollection(api, 'party-directory', '/api/v1/parties'),
    /返回了重复游标/,
  );
});

test('offset collection fails closed when an owner repeats a page', async () => {
  const api = async () => ({ items: [{ id: 'same-item' }], total: 3, has_more: true });

  await assert.rejects(
    loadOffsetCollection(api, 'money-flow-ledger', '/api/v1/cash-transactions'),
    /分页返回了重复记录/,
  );
});
