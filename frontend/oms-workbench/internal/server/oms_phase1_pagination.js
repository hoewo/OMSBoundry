(function (root, factory) {
  const pagination = factory();
  if (typeof module === 'object' && module.exports) module.exports = pagination;
  root.OMSPhase1Pagination = pagination;
})(typeof globalThis === 'object' ? globalThis : this, function () {
  function collectionPath(base, params) {
    const query = new URLSearchParams(params);
    return base + '?' + query.toString();
  }

  function mergeCollectionPage(first, items) {
    return {
      ...(first || {}),
      items,
      total: items.length,
      offset: 0,
      limit: items.length,
      has_more: false,
      next_cursor: null,
    };
  }

  async function loadOffsetCollection(api, id, base, params = {}) {
    let offset = 0;
    let first = null;
    const items = [];
    const seen = new Set();
    for (let pageNumber = 0; pageNumber < 10000; pageNumber++) {
      const page = await api(id, collectionPath(base, { ...params, limit: '100', offset: String(offset) }));
      if (!first) first = page;
      const batch = Array.isArray(page.items) ? page.items : [];
      let added = 0;
      for (const item of batch) {
        const key = String(item?.id || '');
        if (!key || !seen.has(key)) {
          items.push(item);
          added++;
          if (key) seen.add(key);
        }
      }
      const total = Number(page.total);
      const next = offset + batch.length;
      if (batch.length === 0 || (Number.isFinite(total) && next >= total) || page.has_more === false) {
        return mergeCollectionPage(first, items);
      }
      if (added === 0) throw new Error(id + ' 分页返回了重复记录，已停止读取');
      if (next <= offset) throw new Error(id + ' 分页没有前进，已停止读取');
      offset = next;
    }
    throw new Error(id + ' 数据页数超过安全上限');
  }

  async function loadCursorCollection(api, id, base, params = {}) {
    let cursor = '';
    let first = null;
    const items = [];
    const seenIDs = new Set();
    const seenCursors = new Set();
    for (let pageNumber = 0; pageNumber < 10000; pageNumber++) {
      const page = await api(id, collectionPath(base, { ...params, limit: '100', ...(cursor ? { cursor } : {}) }));
      if (!first) first = page;
      const batch = Array.isArray(page.items) ? page.items : [];
      for (const item of batch) {
        const key = String(item?.id || '');
        if (!key || !seenIDs.has(key)) {
          items.push(item);
          if (key) seenIDs.add(key);
        }
      }
      const next = String(page.next_cursor || '');
      if (!next) return mergeCollectionPage(first, items);
      if (seenCursors.has(next)) throw new Error(id + ' 返回了重复游标，已停止读取');
      seenCursors.add(next);
      cursor = next;
    }
    throw new Error(id + ' 数据页数超过安全上限');
  }

  return { loadOffsetCollection, loadCursorCollection };
});
