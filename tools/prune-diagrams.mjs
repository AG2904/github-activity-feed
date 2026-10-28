#!/usr/bin/env node
// Keeps one directory per diagram under .archify/.
//
// Archify names each run with a timestamp, so regenerating a diagram adds a
// directory instead of replacing the old one, and the superseded copy stays in
// the tree — and, once committed, in the remote. One diagram is one directory.
//
// Two modes:
//   node tools/prune-diagrams.mjs          delete every run but the newest of
//                                          its kind; tracked copies go through
//                                          `git rm`, so the deletion is staged
//                                          and reaches the remote on the next
//                                          push.
//   node tools/prune-diagrams.mjs --check  report only, exit 1 when a tracked
//                                          diagram has more than one run. This
//                                          is what CI runs.
//
// Exit codes: 0 clean, 1 duplicate runs found (check mode), 2 the check could
// not run (no .archify, unreadable directory).

import { execFileSync } from 'node:child_process';
import { existsSync, readdirSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const archify = resolve(repoRoot, '.archify');
const checkOnly = process.argv.includes('--check');

/** `architecture-file-calls-cli-20261026-185500` -> `architecture-file-calls-cli`. */
const KIND = /^(.*)-(\d{8}-\d{6})$/;

if (!existsSync(archify)) {
  console.error('prune-diagrams: no .archify directory; refusing to report success');
  process.exit(2);
}

/** Directory names, newest run per kind, and every run that is not it. */
function inventory() {
  const runs = [];
  const unknown = [];

  for (const name of readdirSync(archify, { withFileTypes: true })) {
    if (!name.isDirectory()) continue;
    const match = KIND.exec(name.name);
    if (!match) {
      unknown.push(name.name);
      continue;
    }
    runs.push({ name: name.name, kind: match[1], stamp: match[2] });
  }

  const newestByKind = new Map();
  for (const run of runs) {
    const current = newestByKind.get(run.kind);
    // Timestamps are yyyyMMdd-HHmmss, so a string comparison is chronological.
    if (!current || run.stamp > current.stamp) newestByKind.set(run.kind, run);
  }

  const stale = runs.filter((run) => newestByKind.get(run.kind).name !== run.name);
  return { runs, stale, unknown };
}

/** Paths git knows about, so "is this in the remote" is answered by the index. */
function trackedUnderArchify() {
  const out = execFileSync('git', ['ls-files', '-z', '--', '.archify'], {
    cwd: repoRoot,
    encoding: 'utf8',
  });
  return new Set(
    out
      .split('\0')
      .filter(Boolean)
      .map((path) => path.split('/')[1]),
  );
}

const { runs, stale, unknown } = inventory();
const tracked = trackedUnderArchify();
const staleTracked = stale.filter((run) => tracked.has(run.name));
const staleUntracked = stale.filter((run) => !tracked.has(run.name));

for (const name of unknown) {
  console.warn(`prune-diagrams: ignoring "${name}" (no timestamp to order by)`);
}

if (checkOnly) {
  if (staleTracked.length > 0) {
    for (const run of staleTracked) {
      console.error(`.archify/${run.name}: superseded by a newer "${run.kind}" run`);
    }
    console.error(
      `\nprune-diagrams: ${staleTracked.length} superseded diagram run(s) are in the repository and in the remote.`,
    );
    console.error('Run `npm run diagrams:prune`, then commit and push: the deletion has to travel.');
    process.exit(1);
  }
  if (staleUntracked.length > 0) {
    // They cannot reach the remote, so they are not a build failure — but they
    // are the half of the mess that `git status` never shows.
    console.warn(
      `prune-diagrams: ${staleUntracked.length} superseded run(s) on disk are untracked; ` +
        '`npm run diagrams:prune` removes them.',
    );
  }
  console.log(
    staleUntracked.length > 0
      ? `prune-diagrams: ${runs.length} run(s), nothing superseded in the repository.`
      : `prune-diagrams: ${runs.length} run(s), one per diagram, nothing superseded.`,
  );
  process.exit(0);
}

if (stale.length === 0) {
  console.log(`prune-diagrams: ${runs.length} run(s), one per diagram, nothing to remove.`);
  process.exit(0);
}

if (staleTracked.length > 0) {
  // `--cached` touches only the index: it stages the deletion whether the path
  // is committed or merely added, and never refuses because the worktree copy
  // has changes. The directory is removed from disk below, which is what makes
  // the two halves agree.
  try {
    execFileSync(
      'git',
      ['rm', '-r', '-q', '--cached', '--', ...staleTracked.map((run) => `.archify/${run.name}`)],
      { cwd: repoRoot, stdio: 'inherit' },
    );
  } catch {
    console.warn('prune-diagrams: could not stage the deletion; removing from disk only');
  }
}
for (const run of stale) {
  rmSync(resolve(archify, run.name), { recursive: true, force: true });
}

for (const run of stale) {
  const where = tracked.has(run.name) ? 'staged for deletion' : 'removed from disk';
  console.log(`prune-diagrams: .archify/${run.name} (${where})`);
}
console.log(
  `\nprune-diagrams: ${runs.length - stale.length} diagram(s) left. ` +
    (staleTracked.length > 0
      ? 'Commit and push the staged deletions so the remote matches.'
      : 'Nothing was staged; the copies removed were never in the repository.'),
);
