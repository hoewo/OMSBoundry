(() => {
  'use strict';
  // Reference OA Customer Overlay v1: never persisted in the Project Base.
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
  function numberParts(number) {
    const value = String(number || '').trim(), separator = value.indexOf('-');
    if (separator <= 0) return {year: null, sequence: null};
    const head = value.slice(0, separator).trim(), tail = value.slice(separator + 1).trim();
    const year = /\d{4}$/.test(head) ? Number(head.slice(-4)) : null;
    const match = /^(\d+)(?:[A-Za-z]+\d+)?$/.exec(tail);
    const parsed = match ? BigInt(match[1]) : null;
    const sequence = parsed !== null && parsed <= 9223372036854775807n ? parsed : null;
    return {year, sequence};
  }
  function yearGroups(items) {
    const groups = new Map();
    for (const item of items) {
      const key = numberParts(item.project_number).year;
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(item);
    }
    const date = item => item.planned_start_date || item.created_at || '';
    return [...groups].sort(([a], [b]) => a === null ? 1 : b === null ? -1 : b - a).map(([year, projects]) => ({year, projects: projects.sort((a, b) => {
      const x = numberParts(a.project_number).sequence, y = numberParts(b.project_number).sequence;
      return (x === null && y !== null ? 1 : y === null && x !== null ? -1 : x === y ? 0 : y > x ? 1 : -1) || date(b).localeCompare(date(a)) || b.id.localeCompare(a.id);
    })}));
  }
  function listState(value = {}) {
    const text = v => typeof v === 'string' ? v.slice(0, 200) : '';
    const positive = v => Number.isSafeInteger(v) && v > 0 && v <= 1000000;
    return {schema_version: '1', searchText: text(value.searchText), keyword: text(value.keyword),
      mode: value.mode === 'paged' ? 'paged' : 'timeline', page: positive(value.page) ? value.page : 1,
      pageSize: [10, 20, 50, 100].includes(value.pageSize) ? value.pageSize : 10,
      expanded: Array.isArray(value.expanded) ? [...new Set(value.expanded.filter(id => typeof id === 'string' && uuid.test(id)))].slice(0, 500) : [],
      collapsedYears: Array.isArray(value.collapsedYears) ? [...new Set(value.collapsedYears.filter(id => /^(?:\d{1,4}|unassigned)$/.test(String(id))).map(String))].slice(0, 200) : [],
      scrollTop: Number.isFinite(value.scrollTop) && value.scrollTop >= 0 ? Math.min(value.scrollTop, 1e7) : 0};
  }
  function navigationKey(tenant, subject) {
    return uuid.test(tenant) && uuid.test(subject) ? 'reference-oa-project-list-v1:' + tenant + ':' + subject : null;
  }
  function readState(storage, key) {
    try { const value = key && JSON.parse(storage.getItem(key)); return value?.schema_version === '1' ? listState(value) : listState(); }
    catch { return listState(); }
  }
  function saveState(storage, key, value) { try { if (key) storage.setItem(key, JSON.stringify(listState(value))); } catch {} }
  async function readDirectory(request, tenant, keyword, onPage = () => {}) {
    const items = [], seen = new Set();
    let total = null;
    const validate = (value, child = false) => {
      if (!value || !uuid.test(value.id) || value.tenant_id !== tenant || value.status !== 'active' || typeof value.name !== 'string' || typeof value.project_number !== 'string' || !['project', 'package'].includes(value.node_type) || !Array.isArray(value.assignments) || !Array.isArray(value.children) || seen.has(value.id) || (child && value.children.length)) throw new Error('项目目录标识、范围或层级不一致，请重新读取');
      seen.add(value.id);
      for (const assignment of value.assignments) if (assignment.project_id !== value.id || assignment.tenant_id !== tenant || !['business_owner', 'production_owner'].includes(assignment.role_key) || typeof assignment.display_snapshot !== 'string') throw new Error('项目负责人目录不一致');
      for (const nested of value.children) { if (nested.parent_project_id !== value.id) throw new Error('项目包上下文不一致'); validate(nested, true); }
    };
    do {
      const offset = items.length, parameters = new URLSearchParams({offset: String(offset), limit: '100', keyword});
      const page = await request('/api/v1/project-directory?' + parameters);
      if (page.schema_version !== '1' || !Array.isArray(page.items) || page.items.length > 100 || page.offset !== offset || page.limit !== 100 || !Number.isSafeInteger(page.total) || page.total < 0 || typeof page.has_more !== 'boolean' || (total !== null && page.total !== total)) throw new Error('项目目录在读取中发生变化，请重新读取');
      total = page.total;
      for (const item of page.items) validate(item);
      items.push(...page.items);
      if (items.length > total || page.has_more !== (items.length < total) || (page.has_more && !page.items.length)) throw new Error('项目分页未完整返回，请重新读取');
      onPage(items.length, total);
      if (!page.has_more) return items;
    } while (true);
  }
  globalThis.ReferenceOAProjectDirectory = {numberParts, yearGroups, listState, navigationKey, readState, saveState, readDirectory};
})();
