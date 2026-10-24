#!/usr/bin/env node
// Builds the feed binary: the host platform by default, or the four distributed
// targets with --dist. A Node script rather than a shell line because the
// targets need environment variables, and `CGO_ENABLED=0 GOOS=linux go build`
// is not a command any Windows shell understands; Node is already here because
// npm is what runs this.
//
// Both modes build the same way: CGO_ENABLED=0 and -trimpath, so what a
// developer runs is what a teammate downloads, and the version comes from
// -ldflags so a distributed binary can say where it came from.
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const tools = dirname(fileURLToPath(import.meta.url));
const client = dirname(tools);
const repo = dirname(client);

const distributed = [
  { goos: 'windows', goarch: 'amd64' },
  { goos: 'linux', goarch: 'amd64' },
  { goos: 'darwin', goarch: 'amd64' },
  { goos: 'darwin', goarch: 'arm64' },
];

const platforms = { win32: 'windows', linux: 'linux', darwin: 'darwin' };
const architectures = { x64: 'amd64', arm64: 'arm64' };

const targets = process.argv.includes('--dist')
  ? distributed
  : distributed.filter(
      (target) =>
        target.goos === platforms[process.platform] && target.goarch === architectures[process.arch],
    );

if (targets.length === 0) {
  const known = distributed.map((target) => `${target.goos}/${target.goarch}`).join(', ');
  console.error(`feed: no target for ${process.platform}/${process.arch}. The four are: ${known}`);
  process.exit(1);
}

// The version a distributed binary reports: an explicit FEED_VERSION wins, so the
// release workflow can pass the tag without anyone hand-editing package.json; a
// local build falls back to the package's own version plus the commit it was
// built from when there is a repository to ask. A tarball without git still
// builds; it just cannot say more than the package version.
const version = () => {
  const explicit = (process.env.FEED_VERSION ?? '').trim();
  if (explicit) {
    return explicit;
  }
  const { version: packageVersion } = JSON.parse(readFileSync(join(repo, 'package.json'), 'utf8'));
  const described = git(['describe', '--always', '--dirty', '--abbrev=7']);
  return described ? `${packageVersion}+${described}` : packageVersion;
};

const git = (args) => {
  try {
    return execFileSync('git', args, { cwd: repo, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
};

const stamp = version();
const out = join(client, 'dist');
mkdirSync(out, { recursive: true });

for (const target of targets) {
  const name = `feed-${target.goos}-${target.goarch}${target.goos === 'windows' ? '.exe' : ''}`;

  execFileSync(
    'go',
    ['build', '-trimpath', `-ldflags=-X main.version=${stamp}`, '-o', join(out, name), './cmd/feed'],
    {
      cwd: client,
      stdio: 'inherit',
      env: { ...process.env, CGO_ENABLED: '0', GOOS: target.goos, GOARCH: target.goarch },
    },
  );

  console.log(`feed: dist/${name}  ${target.goos}/${target.goarch}  ${stamp}`);
}
