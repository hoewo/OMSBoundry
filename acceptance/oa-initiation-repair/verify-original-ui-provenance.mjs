#!/usr/bin/env node
// Read-only source snapshot check. It does not prove adapted UI equivalence.
import { readFile, realpath } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { dirname, isAbsolute, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const source = process.argv[2];
if (!source || process.argv.length !== 3) throw new Error('provide the original frontend source directory');
const sourceRoot = await realpath(resolve(source));
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const provenance = JSON.parse(await readFile(join(repoRoot, 'frontend/oms-workbench/frontend/source-provenance.json'), 'utf8'));
if (!Array.isArray(provenance.files) || provenance.files.length === 0) throw new Error('missing provenance records');
const seen = new Set();
for (const item of provenance.files) {
  if (typeof item.path !== 'string' || isAbsolute(item.path) || item.path.split('/').some(p => p === '..' || !p) || seen.has(item.path)) throw new Error('invalid or duplicated source path');
  if (!/^[a-f0-9]{64}$/.test(item.sha256)) throw new Error('invalid source digest');
  seen.add(item.path);
  const file = await realpath(join(sourceRoot, item.path));
  const inside = relative(sourceRoot, file);
  if (inside === '..' || inside.startsWith('..' + sep) || isAbsolute(inside)) throw new Error('source path escaped explicit root');
  const digest = createHash('sha256').update(await readFile(file)).digest('hex');
  if (digest !== item.sha256) throw new Error('original source drift: ' + item.path);
}
console.log(JSON.stringify({ passed: true, check: 'original_ui_source_snapshot_digests', copied_files: seen.size, source_writes: 0, adapted_ui_equivalence: false, public_ui_verified: false }));
