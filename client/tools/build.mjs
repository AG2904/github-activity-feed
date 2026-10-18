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
const architectures = { x64: 'amd64', arm64: 'arm64', ia32: '386' };

const host = { goos: platforms[process.platform], goarch: architectures[process.arch] };
const targets = process.argv.includes('--dist')
  ? distributed
  : distributed.filter((target) => target.goos === host.goos && target.goarch === host.goarch);

if (targets.length === 0) {
  console.error(`feed: no target for ${host.goos}/${host.goarch}`);
  process.exit(1);
}

// The version a distributed binary reports: the package's own, plus the commit
// it was built from when there is a repository to ask. A tarball without git
// still builds; it just cannot say more than the package version.
const version = () => {
  const { version } = JSON.parse(readFileSync(join(repo, 'package.json'), 'utf8'));
  const described = git(['describe', '--always', '--dirty', '--abbrev=7']);
  return described ? `${version}+${described}` : version;
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
