# Ticket 01 - Ingesta y autenticación (walking skeleton)

## Resumen

El Relay arranca, expone una única seam HTTP para webhooks de GitHub y rechaza
cualquier request cuya firma no sea válida. No sobrevive ningún evento todavía:
nada se filtra, normaliza ni almacena. El ticket levanta el esqueleto — la app,
el wiring de persistencia, la seam HTTP y el harness de tests — para que todos
los tickets siguientes tengan dónde aterrizar.

El ticket se implementó en dos etapas: el skeleton original (PR #1, autenticación
con `X-Gitlab-Token`) y la migración a GitHub (PR #3, autenticación HMAC).

## Cambios Realizados

### src/app.module.ts

- Wiring de **TypeORM sobre SQLite embebido** (ADR-0004) con `forRootAsync`, para
  que la ruta de la base se lea en el momento de inicialización y no al importar
  el módulo: `better-sqlite3`, archivo `DATABASE_PATH` (default `relay.db`).
- **Fail-fast del secreto**: el constructor rechaza el arranque si
  `GITHUB_WEBHOOK_SECRET` no está definido. La app nunca corre en un modo en el
  que aceptaría sentencias sin secreto.

### src/main.ts

- `NestFactory.create(AppModule, { rawBody: true })`: captura del cuerpo crudo
  necesaria para verificar el HMAC de GitHub sobre los bytes exactos firmados.

### src/relay/webhook.controller.ts

- `POST /`: verificación de `X-Hub-Signature-256` — HMAC-SHA256 del cuerpo crudo,
  hex, prefijo `sha256=`.
- **Comparación en tiempo constante** (`timingSafeEqual` sobre digests de longitud
  fija): no cortocircuita en el primer byte distinto ni filtra la longitud del
  secreto.
- **Fail-closed**: firma ausente, malformada, prefijo incorrecto, secreto
  vacío o cuerpo ausente → `401`.

### src/relay/history.controller.ts

- `GET /history`: colección de historial de auditoría (vacía mientras nada se
  almacene).

### src/relay/relay.module.ts

- Módulo del Relay con las dos controllers de la seam.

### test/app.e2e-spec.ts

- Suite e2e sobre la **única seam**: la app completa bootea contra un SQLite
  temporal (`DATABASE_PATH` a un directorio temporal) y todo se ejerce por HTTP
  con `supertest` — ningún test toca handlers, parser ni ORM directamente.
- Casos de rechazo: sin firma, secreto incorrecto, cuerpo que no coincide con su
  firma, digest no-hex, header malformado. Caso positivo: firma válida — incluido
  un cuerpo con whitespace no canónico firmado byte a byte, que es lo único que
  prueba que el HMAC es sobre bytes crudos y no sobre un body re-serializado.

### Configuración y harness

- `vitest.config.e2e.ts`, `tsconfig.json` (`types: ["vitest/globals"]`), scripts
  npm (`build`/`lint`/`format`/`start*`/`test`), `.env.example`,
  `.gitignore` (`node_modules/`, `dist/`, `.env`, `*.tsbuildinfo`, alias locales).

## Comportamiento resultante

- La app bootea contra un archivo SQLite temporal, sin servicios externos, y
  rechaza el arranque si falta el secreto.
- Un request sin firma válida recibe `401`; uno correctamente firmado recibe
  `200 { received: true }`.
- `GET /history` responde una colección (vacía antes de que algo se almacene).
- El secreto vive solo en el entorno: nunca en código commiteado.

## Verificación

```text
npm test        → 8 casos e2e en verde (post-migración HMAC)
tsc --noEmit    → exit 0
oxlint          → 0 warnings, 0 errors
```

Fail-fast verificado además en proceso real: `node dist/main` sin el secreto
registra `Error: GITHUB_WEBHOOK_SECRET is not set; the relay refuses to boot.`
y termina sin abrir puerto ni base.

## Acceptance Criteria

- ✅ AC1: La app bootea contra un SQLite temporal, sin servicios externos.
- ✅ AC2: Un único endpoint HTTP POST recibe los webhooks de GitHub.
- ✅ AC3: Sin firma, firma malformada o firmada con el secreto equivocado → `401`.
- ✅ AC4: Cuerpo que no coincide con su firma (alterado) → `401`.
- ✅ AC5: Firma HMAC-SHA256 válida sobre el cuerpo crudo → éxito.
- ✅ AC6: El secreto se lee del entorno, nunca de código commiteado.
- ✅ AC7: La comparación de firma no cortocircuita en el primer byte distinto.
- ✅ AC8: Existe el endpoint de historial y devuelve una colección vacía.
- ✅ AC9: Un test de integración ejerce el endpoint por HTTP, no llamando al
  handler directamente.
