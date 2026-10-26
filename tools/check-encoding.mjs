#!/usr/bin/env node
// Fails when a tracked markdown file is not plain UTF-8.
//
// Why this exists: four files under .scratch/ were committed as doubly encoded
// UTF-8 — "sólo" stored as its Latin-1 reading — and every other check passed.
// A grep for the word cannot match a broken word, so the damage is invisible to
// the tool that should have caught it, and invisible to the eye on a terminal
// that renders the bytes as mojibake anyway. Encoding is checked on bytes, by
// something that looks at bytes.
//
// Run with no arguments to check the markdown files git tracks; pass paths to
// check specific files. Before scanning, the detector proves itself against
// samples: a check that cannot fail is not a check.
//
// Exit codes: 0 clean, 1 at least one file is not plain UTF-8, 2 the check could
// not run at all (git missing, no files, or a detector that stopped working) — a
// broken check must never pass silently.

import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');

/**
 * The shapes a UTF-8 byte sequence takes when something reads it as Latin-1.
 * Each pair is a lead character plus the range its continuation byte lands in:
 * an accented vowel becomes "Ã" + a punctuation mark, an em dash becomes
 * "â" + "€" + a quote, "§" becomes "Â" + "§". Matching the pair rather than the
 * lone character keeps real text that happens to contain "Â" or "Ã" green.
 */
const MOJIBAKE = [
  /Ã[\u0080-\u00bf]/u,
  /â€[\u0080-\u00bf\u2018\u2019\u201c\u201d\u2122\u0153]/u,
  /Â[\u0080-\u00bf]/u,
];

const decoder = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true });

/** Markdown files git tracks, so a build directory or a scratch copy is never scanned. */
function trackedMarkdown() {
  const out = execFileSync('git', ['ls-files', '--', '*.md'], {
    cwd: repoRoot,
    encoding: 'utf8',
  });
  return out.split('\n').filter(Boolean);
}

/** Every problem in one file's bytes, each with the line it sits on. */
function problemsIn(path, bytes) {
  const problems = [];

  let text;
  try {
    text = decoder.decode(bytes);
  } catch {
    return [{ path, line: 0, detail: 'is not valid UTF-8 (a byte sequence cannot be decoded)' }];
  }

  // ignoreBOM above keeps the marker in the string instead of eating it, which
  // is what lets it be reported. A BOM is not corruption, but it is the
  // signature of the Windows tooling that writes one, so it is worth refusing.
  if (text.charCodeAt(0) === 0xfeff) {
    problems.push({ path, line: 1, detail: 'starts with a UTF-8 BOM' });
  }

  text.split('\n').forEach((line, index) => {
    if (line.includes('\ufffd')) {
      problems.push({
        path,
        line: index + 1,
        detail: 'contains U+FFFD, the replacement character (bytes were lost before this was saved)',
      });
    }
    for (const re of MOJIBAKE) {
      if (re.test(line)) {
        problems.push({ path, line: index + 1, detail: 'is UTF-8 read as Latin-1' });
      }
    }
  });

  return problems;
}

/**
 * Reproduces the bug that motivated this file: the UTF-8 bytes decoded as
 * Latin-1 and then saved again as UTF-8. Deriving the sample instead of typing
 * it keeps this source free of the corruption it looks for.
 */
const doubleEncoded = (text) => Buffer.from(Buffer.from(text, 'utf8').toString('latin1'), 'utf8');

/**
 * Proves the detector still detects. Run on every invocation, because a check
 * that has quietly stopped matching is indistinguishable from a clean repo, and
 * that is how four broken files shipped in the first place.
 */
function assertDetectorWorks() {
  const samples = [
    { name: 'clean text with accents', bytes: Buffer.from('sólo, aquí, año, —, §', 'utf8'), want: 0 },
    { name: 'mojibake (the original bug)', bytes: doubleEncoded('sólo'), want: 1 },
    { name: 'replacement character', bytes: Buffer.from('a \ufffd b', 'utf8'), want: 1 },
    { name: 'byte order mark', bytes: Buffer.concat([Buffer.from([0xef, 0xbb, 0xbf]), Buffer.from('# t')]), want: 1 },
    { name: 'invalid UTF-8', bytes: Buffer.from([0x74, 0xff]), want: 1 },
  ];

  const broken = samples.filter(({ bytes, want }) => problemsIn('sample', bytes).length !== want);
  if (broken.length > 0) {
    for (const { name } of broken) {
      console.error(`check-encoding: the detector no longer handles "${name}"`);
    }
    console.error('check-encoding: refusing to report on real files with a broken detector');
    process.exit(2);
  }
}

assertDetectorWorks();

const paths = process.argv.length > 2 ? process.argv.slice(2) : trackedMarkdown();
if (paths.length === 0) {
  console.error('check-encoding: no markdown files to check; refusing to report success');
  process.exit(2);
}

const problems = paths.flatMap((path) => {
  try {
    return problemsIn(path, readFileSync(resolve(repoRoot, path)));
  } catch (error) {
    return [{ path, line: 0, detail: `cannot be read: ${error.message}` }];
  }
});

if (problems.length > 0) {
  for (const { path, line, detail } of problems) {
    console.error(`${path}:${line}: ${detail}`);
  }
  const files = new Set(problems.map((problem) => problem.path));
  console.error(
    `\ncheck-encoding: ${problems.length} problem(s) in ${files.size} file(s) of ${paths.length} checked.`,
  );
  console.error('Fix the bytes, not the text: re-save the file as UTF-8, or decode it from Latin-1 once.');
  process.exit(1);
}

console.log(`check-encoding: ${paths.length} markdown file(s) are plain UTF-8.`);
