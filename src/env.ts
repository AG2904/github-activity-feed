import { existsSync, readFileSync } from 'node:fs';
import { parseEnv } from 'node:util';

const UTF8_BOM = '\uFEFF';

/**
 * Load a `.env` file into `process.env` using Node's built-in parser.
 *
 * Precedence: variables already present in the real environment always win, so
 * a shell export overrides the file. A missing file is not an error — it just
 * means there is nothing to load.
 *
 * The file is read here instead of calling `process.loadEnvFile` directly only
 * to strip a UTF-8 BOM: Windows tooling (PowerShell 5.1 `Set-Content -Encoding
 * utf8`) writes one, and the loader would otherwise absorb it into the first
 * variable name, silently losing that variable.
 */
export function loadEnvFile(path: string = '.env'): void {
  if (!existsSync(path)) {
    return;
  }

  const raw = readFileSync(path, 'utf8');
  const content = raw.startsWith(UTF8_BOM) ? raw.slice(1) : raw;

  for (const [key, value] of Object.entries(parseEnv(content))) {
    if (process.env[key] === undefined) {
      process.env[key] = value;
    }
  }
}
