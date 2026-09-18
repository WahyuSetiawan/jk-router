#!/usr/bin/env node
// JKRouter knowledge-graph auto-update (deterministic portion of the /understand auto-update flow).
// Read-only for git; writes .ua/knowledge-graph.json + .ua/meta.json.
// Usage: node scripts/ua-auto-update.mjs [--full]
//
// ponytail: only re-LLM-analyzes source files that need it; cosmetic/config changes are skipped
// or get a graph-merge without LLM. FULL LLM re-analysis (new batches) is still /understand or
// an agent session — this script keeps the fingerprint + meta + prune bookkeeping correct.
import { readFileSync, writeFileSync, existsSync, mkdirSync, rmSync, cpSync } from 'node:fs';
import { execSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = process.cwd();
const UA = path.join(root, '.ua');
const GRAPH = path.join(UA, 'knowledge-graph.json');
const META = path.join(UA, 'meta.json');
const INTER = path.join(UA, 'intermediate');
const FULL = process.argv.includes('--full');

if (!existsSync(GRAPH)) { console.log('[ua-auto-update] no knowledge-graph.json — run /understand first.'); process.exit(0); }
const meta = JSON.parse(readFileSync(META, 'utf8'));
const head = execSync('git rev-parse HEAD', { encoding: 'utf8' }).trim();
if (head === meta.gitCommitHash && !FULL) { console.log(`[ua-auto-update] up to date at ${head.slice(0, 8)} — nothing to do.`); process.exit(0); }

const changed = execSync(`git diff ${meta.gitCommitHash}..HEAD --name-only`, { encoding: 'utf8' })
  .split('\n').map(s => s.trim()).filter(Boolean)
  .filter(p => !p.startsWith('.ua/') && !p.startsWith('jkserver/cmd/_nuxt/'));
if (changed.length === 0) {
  meta.gitCommitHash = head; meta.lastAnalyzedAt = new Date().toISOString();
  writeFileSync(META, JSON.stringify(meta, null, 2));
  console.log('[ua-auto-update] only .ua/_nuxt paths changed — meta refreshed, graph untouched.');
  process.exit(0);
}

// Rebuild fingerprints baseline for the current file set so the next run classifies correctly.
const SRC_EXT = /\.(ts|tsx|js|jsx|mjs|py|go|rs|java|rb|cpp|c|h|cs|swift|kt|php|sql|vue|nix|yaml|yml|json|css|html|sh|md)$/;
const srcChanged = changed.filter(p => SRC_EXT.test(p));
const graph = JSON.parse(readFileSync(GRAPH, 'utf8'));

if (srcChanged.length === 0) {
  meta.gitCommitHash = head; meta.lastAnalyzedAt = new Date().toISOString();
  writeFileSync(META, JSON.stringify(meta, null, 2));
  console.log(`[ua-auto-update] ${changed.length} non-source files changed — meta refreshed, graph untouched.`);
  process.exit(0);
}

// Prune graph nodes for removed/renamed source files, keep everything else.
const gone = new Set(srcChanged.filter(p => !existsSync(path.join(root, p))));
if (gone.size) {
  const before = graph.nodes.length;
  graph.nodes = graph.nodes.filter(n => !gone.has(n.path));
  const ids = new Set(graph.nodes.map(n => n.id));
  graph.edges = graph.edges.filter(e => ids.has(e.source) && ids.has(e.target));
  (graph.layers || []).forEach(l => { l.nodeIds = (l.nodeIds || []).filter(id => ids.has(id)); });
  (graph.tour || []).forEach(t => { t.nodeIds = (t.nodeIds || []).filter(id => ids.has(id)); });
  console.log(`[ua-auto-update] pruned ${before - graph.nodes.length} nodes for ${[...gone].join(', ')}`);
}

graph.project.analyzedAt = new Date().toISOString();
graph.project.gitCommitHash = head;
writeFileSync(GRAPH, JSON.stringify(graph, null, 2));

// Refresh fingerprints baseline (90-file deterministic cost, no LLM).
try {
  const skillDir = '/home/juragankoding/.understand-anything/repo/understand-anything-plugin/skills/understand';
  const inputPath = path.join(INTER, 'fingerprint-input.json');
  mkdirSync(INTER, { recursive: true });
  let sourceFilePaths = [];
  const scanPath = path.join(INTER, 'scan-result.json');
  if (existsSync(scanPath)) sourceFilePaths = JSON.parse(readFileSync(scanPath, 'utf8')).files.map(f => f.path);
  if (sourceFilePaths.length === 0) {
    // ponytail: rescan fallback omitted — regenerate scan-result.json via /understand if it goes missing
    sourceFilePaths = srcChanged;
  }
  writeFileSync(inputPath, JSON.stringify({ projectRoot: root, sourceFilePaths, gitCommitHash: head }, null, 2));
  execSync(`node "${skillDir}/build-fingerprints.mjs" "${inputPath}"`, { stdio: 'inherit' });
} catch (err) {
  console.error('[ua-auto-update] fingerprint refresh failed:', err.message, '— graph + meta still updated.');
}

meta.gitCommitHash = head; meta.lastAnalyzedAt = new Date().toISOString(); meta.analyzedFiles = graph.nodes.length;
writeFileSync(META, JSON.stringify(meta, null, 2));
console.log(`[ua-auto-update] graph refreshed: ${graph.nodes.length} nodes, ${graph.edges.length} edges, ${srcChanged.length} source files changed since ${meta.gitCommitHash.slice(0, 8)}.`);
console.log('[ua-auto-update] if structure changed materially (new files/modules), run /understand for a full LLM pass.');
