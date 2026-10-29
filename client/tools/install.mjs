#!/usr/bin/env node
// Installs the host build of the feed binary as a command: build, then copy it to
// the user's own bin directory under the short name.
//
// Three rules, and nothing else:
//
//   1. It grants no permissions. Windows has no executable bit, and a Go build
//      already emits an executable file on the platforms that do; copying keeps
//      the mode. Nothing here runs elevated.
//   2. It does not touch PATH. It reports where the binary went and, when that
//      directory is not on PATH, says so — reading the environment, never writing
//      it. Editing someone's PATH from an npm script is a change nobody remembers
//      making.
//   3. It does not overwrite a binary it did not build. A different file under the
//      target name is a refusal that asks for --force, so an unrelated `feed`
//      already living there is never clobbered.
//
// Two rules the copy itself follows, because writing beside a name is not the same
// as writing through it. A file that is a link is refused even under --force:
// --force replaces a file, it never agrees to follow a link somewhere else. And the
// new binary is written under a temporary name and renamed over the target, so the
// replacement is atomic and cannot be redirected by a link that appears between the
// checks and the write.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import {
  copyFileSync,
  existsSync,
  lstatSync,
  mkdirSync,
  readFileSync,
  renameSync,
  rmSync,
} from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const tools = dirname(fileURLToPath(import.meta.url));
const client = dirname(tools);
const windows = process.platform === 'win32';

const platforms = { win32: 'windows', linux: 'linux', darwin: 'darwin' };
const architectures = { x64: 'amd64', arm64: 'arm64' };

const goos = platforms[process.platform];
const goarch = architectures[process.arch];

if (!goos || !goarch) {
  console.error(`feed: no install target for ${process.platform}/${process.arch}.`);
  process.exit(1);
}

const name = windows ? 'feed.exe' : 'feed';
const built = join(client, 'dist', `feed-${goos}-${goarch}${windows ? '.exe' : ''}`);
const targetDirectory = join(homedir(), 'bin');
const target = join(targetDirectory, name);
const force = process.argv.includes('--force');

const digest = (file) => createHash('sha256').update(readFileSync(file)).digest('hex');

execFileSync(process.execPath, [join(tools, 'build.mjs')], { cwd: client, stdio: 'inherit' });

if (!existsSync(built)) {
  console.error(`feed: the build did not produce ${built}.`);
  process.exit(1);
}

if (existsSync(target)) {
  // lstat, not stat: the question is what sits under that name, not what it points
  // at. Following the link first would already be the bug.
  const stats = lstatSync(target);

  if (stats.isSymbolicLink()) {
    console.error(
      `feed: ${target} is a symbolic link. Refusing to write through it.\n` +
        'Remove it yourself and run again; --force does not follow links.',
    );
    process.exit(1);
  }

  if (stats.nlink > 1) {
    console.error(
      `feed: ${target} has ${stats.nlink} hard links, so writing it would change the ` +
        'other one too.\nRemove it yourself and run again.',
    );
    process.exit(1);
  }

  if (digest(target) !== digest(built) && !force) {
    console.error(
      `feed: ${target} already exists and is not the binary this build produced.\n` +
        'Refusing to overwrite it. Re-run with --force if that file is yours to replace.',
    );
    process.exit(1);
  }
}

mkdirSync(targetDirectory, { recursive: true });

// Write beside the target and rename over it: the replacement is one operation, and
// nothing can appear between the checks above and the write to redirect it.
const staging = join(targetDirectory, `${name}.${process.pid}.tmp`);
try {
  copyFileSync(built, staging);
  renameSync(staging, target);
} finally {
  rmSync(staging, { force: true });
}

const version = execFileSync(target, ['--version'], { encoding: 'utf8' }).trim();
console.log(`feed: installed ${version} at ${target}`);

if (!(process.env.PATH ?? '').split(delimiter).includes(targetDirectory)) {
  console.log(
    `feed: ${targetDirectory} is not on this shell's PATH — a new terminal sees it once it is on your user PATH.`,
  );
}
