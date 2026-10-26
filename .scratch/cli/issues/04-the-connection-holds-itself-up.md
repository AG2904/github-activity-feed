# 04: The connection holds itself up

**What to build:** The reconnect and liveness behaviour that lets the CLI be left running in a terminal for days.

**Blocked by:** 02

**Status:** done

- [x] A dropped connection is reopened without intervention, backing off exponentially from one second to a thirty-second ceiling, with jitter.
- [x] The backoff resets as soon as an event arrives.
- [x] A stream that goes silent past five minutes is treated as dead and reopened.
- [x] Every reconnect attempt writes exactly one line to stderr; stdout carries events and nothing else.
- [x] A relay that is unreachable at startup is retried indefinitely rather than being fatal.
- [x] A reconnect never replays: no event already printed is printed again.
- [x] Ctrl-C exits 0 after flushing, with no stack trace.

## Comments

- 2026-10-17: The relay sent no heartbeat when this ticket was written, so a half-open connection — the corporate proxy that drops a stream in silence — looked alive forever, and the client could only assume death after five minutes and reopen. A false positive is cheap: reconnecting replays nothing, so nothing is lost or duplicated. Relay ticket 08 has since landed a heartbeat, so the timeout is now a backstop rather than the primary signal: it covers a connection that dies inside a heartbeat interval, and a relay that stops writing without closing the socket. The rule does not change — the client still assumes death after five minutes of silence, because it cannot see the relay's timer and a quiet relay still looks like a gone one between beats.
- 2026-10-18: Completed (19 process-seam cases, `go vet`/`gofmt` clean, linux/darwin builds clean, relay suite still 54 green). Two commits on `main`: `68e10f9 feat: keep the feed alive through drops and silence (ticket 04)` and `454edf1 fix: keep reconnect honest and local failures fatal (ticket 04)`, el segundo con las correcciones de la revisión.

  **El loop y la conexión.** `watch` vive ahora en `watch.go`: un loop que abre una conexión, la sigue hasta que termina o se calla, y decide si reintentar. Cada intento fallido escribe exactamente una línea en stderr con la espera y el motivo (`watch: reconnecting in 1.12s: connection closed`). Cuando una conexión vuelve a establecerse después de una caída, escribe `watch: reconnected`; no es una línea de intento, es lo que permite distinguir recuperación de silencio (story 9). stdout no cambia: solo eventos.

  **Backoff.** Escalones de 1, 2, 4, 8, 16 y 30 segundos, con jitter de hasta un cuarto del escalón y clamp al techo de 30s. Un evento emitido resetea el backoff antes de calcular la espera siguiente. Un relay inalcanzable al arrancar se reintenta indefinidamente.

  **Liveness.** `idleReader` resetea un timer en cada byte leído, no en cada evento: los heartbeats del relay son frames de comentario y cuentan. Si pasan cinco minutos sin un byte, cierra el cuerpo, la lectura se desbloquea y el loop reconecta con el motivo `no bytes for 5m0s`.

  **Fallos locales vs reintentables.** Una URL inválida o un stdout cerrado son fatales y salen con 1; una conexión caída, un non-200 o un stream que termina se reintentan. Sin esa distinción, `feed watch --relay ://bad` reintentaba para siempre.

  **Ctrl-C.** `main` instala `signal.NotifyContext`; el primer Ctrl-C cancela el request o la espera y `watch` sale con 0. Apenas se cancela, se restaura el handler por defecto, así que un segundo Ctrl-C puede matar un proceso trabado.

  **Test seam.** El caso de silencio se prueba con un segundo binario compilado con `-ldflags -X ...idleTimeoutSetting=300ms`; el binario distribuido conserva los cinco minutos. Es el mismo código y sigue siendo el límite del proceso, aunque el default de producción no se espere en un test.

  **Revisión.** Corregido: el primer backoff pasó de 0.5–1s a 1–1.25s (el ticket dice "desde un segundo"), los errores locales ya no se reintentan, el handshake del fake relay pasó a canales sin buffer para que una señal pertenezca a la conexión actual, y se agregaron el test de heartbeats, la aserción de `Last-Event-ID` vacío en cada reconexión y el conteo exacto de líneas por intento.

  **Límites conocidos.** El camino de Ctrl-C no está automatizado: en Windows no hay una forma portable de mandar un interrupt a un proceso hijo desde el test. El techo de 30s y el default de cinco minutos no se esperan de verdad; se verifica el mismo código con escalones y timeout cortos.
