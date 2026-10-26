import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { loadEnvFile } from '../src/env.js';

const FROM_FILE = 'RELAY_ENV_TEST_FROM_FILE';
const SHELL_WINS = 'RELAY_ENV_TEST_SHELL_WINS';

describe('loadEnvFile', () => {
  let directory: string;

  beforeAll(() => {
    directory = mkdtempSync(join(tmpdir(), 'relay-env-'));
  });

  afterAll(() => {
    rmSync(directory, { recursive: true, force: true });
  });

  afterEach(() => {
    delete process.env[FROM_FILE];
    delete process.env[SHELL_WINS];
  });

  it('loads variables from the file', () => {
    const path = join(directory, 'load.env');
    writeFileSync(path, `${FROM_FILE}=loaded\n`);

    loadEnvFile(path);

    expect(process.env[FROM_FILE]).toBe('loaded');
  });

  it('does not override variables already set in the environment', () => {
    const path = join(directory, 'precedence.env');
    writeFileSync(path, `${SHELL_WINS}=from_file\n`);
    process.env[SHELL_WINS] = 'from_shell';

    loadEnvFile(path);

    expect(process.env[SHELL_WINS]).toBe('from_shell');
  });

  it('is a no-op when the file is missing', () => {
    expect(() => loadEnvFile(join(directory, 'absent.env'))).not.toThrow();
  });

  it('strips a UTF-8 BOM so the first variable keeps its name', () => {
    const path = join(directory, 'bom.env');
    writeFileSync(path, `\uFEFF${FROM_FILE}=bom\n`);

    loadEnvFile(path);

    expect(process.env[FROM_FILE]).toBe('bom');
  });
});
