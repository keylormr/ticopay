# Tico Pay — Roadmap

Estado y pendientes de Tico Pay (pagos CR full-stack). Pensado para retomar en una sesión nueva.

- **Frontend:** https://ticopay.vercel.app · **API:** https://ticopay.onrender.com
- **Stack:** Go (chi + pgx + JWT) en Render · React + Vite (TS) en Vercel · Postgres en Neon
- **Deploy:** `git push origin main` → Render y Vercel auto-despliegan (repo **público** `kryrmz/ticopay`)
- **Cuenta demo:** `maria@ticopay.cr` / `password123`

---

## ✅ Hecho y desplegado (verificado)
Auth (clave + **passkeys/WebAuthn** passwordless + **códigos de recuperación** de un solo uso) · multimoneda (₡, $, €, MXN + 15 cripto con precios CoinGecko + tipo de cambio BCCR) · **enviar** por teléfono/correo · **SINPE Móvil** simulado (con comprobante) · **convertir** entre cualquier par · **cobros** (con QR/WhatsApp) · **vaquitas** · **pago de servicios** (ICE, AyA, marchamo, RTV, CCSS…) · **KYC** cédula/DIMEX · patrimonio estimado · UI amigable con pestañas · **i18n ES/EN** (UI + errores del backend) · **rate-limiting + bloqueo de cuenta** (5 intentos → 15 min).

> ⚠️ **Importante:** cripto, servicios y SINPE son un **libro contable interno (simulado)** — no mueven plata real ni liquidan con blockchain/ICE/INS/SINPE real.

---

## 🟡 Pendientes — Seguridad / producción

### 1. Recuperar contraseña  *(necesita proveedor de correo)*
- **Bloqueador:** entregar el enlace/código requiere email (Resend o SendGrid, plan gratis → API key en env de Render).
- **Approach:** migración `password_reset_tokens` (token hasheado + expiración ~30 min); `POST /api/auth/forgot` (genera token, manda email) y `POST /api/auth/reset` (valida token, cambia hash). Front: enlace "¿Olvidaste tu contraseña?" en `AuthPage.tsx`.

### 2. ~~Códigos de recuperación de passkey~~ ✅ **Hecho, desplegado y verificado en prod**
- 10 códigos de un solo uso (formato `XXXX-XXXX`, alfabeto sin glifos ambiguos), hasheados con bcrypt en `passkey_recovery_codes` (migración `0007`). Se muestran UNA sola vez; regenerar invalida los anteriores.
- Backend: `recovery.go` → `GET/POST /api/passkeys/recovery-codes` (autenticado) y `POST /api/auth/recovery` (login con código, comparte el bloqueo por intentos de `hardening.go`, key `recovery:<email>`). Tests en `recovery_test.go`.
- Front: `sections/Account.tsx` (sección "🛟 Códigos de recuperación": estado/generar/regenerar/copiar) y `pages/AuthPage.tsx` (enlace "¿Perdiste tu llave?" → entrar con código). i18n ES/EN agregado.
- Verificado E2E contra prod: generar → entrar con código → el código se consume (reuso da 401, `remaining` baja). ✓

### 3. Verificación de correo al registrarse  *(necesita correo, igual que #1)*
### 4. ~~2FA TOTP como alternativa a passkeys~~ ✅ **Hecho**
- Migración `0009_totp.sql` (tabla `user_totp`: secreto por usuario, gate solo si `confirmed`). Backend `totp.go`: `GET /api/totp` (estado), `POST /api/totp/setup` (secreto + otpauth URL), `/confirm` (valida 1er código y activa), `/disable` (pide código válido). Login: con 2FA activo responde **428** si falta `totpCode`; código malo cuenta para el lockout.
- Front: sección "📱 Verificación en dos pasos" en `Account.tsx` (QR con `qrcode.react` + clave manual + confirmar/desactivar); `AuthPage.tsx` muestra campo de código al recibir 428. i18n ES/EN.

---

## 🔵 Pendientes — Pulido
- ✅ ~~**Nombre personalizado** del passkey al registrar~~ — input opcional en `sections/Account.tsx` (default localizado si va vacío). **Desplegado.**
- ✅ ~~Traducir los **errores 500 técnicos**~~ — mapa `errsES` en `internal/api/i18n.go` (español es el idioma por defecto). **Desplegado y verificado.**
- ✅ ~~**Más fiat** (EUR, MXN)~~ — catálogo + feed FX `frankfurter.app` (`usdPerUnit` con caché/fallback), migración `0008` hace backfill de cuentas a usuarios existentes. **Desplegado y verificado** (EUR≈1.15, MXN≈0.057; convert USD→EUR ok). Para agregar más (GBP, CAD…): solo sumar al catálogo `currency.go` + `currencies.ts` + `format.ts` y una migración de backfill.
- **Quitar `RUN_MIGRATIONS` y `SEED_DEMO`** de las env vars de Render (ya corrieron; son idempotentes). *Pendiente: cambio en el dashboard de Render, no en código. Nota: dejar `RUN_MIGRATIONS=true` no hace daño y permite que futuras migraciones corran solas.*

---

## 🟢 Roadmap mayor (lo que lo haría imbatible en CR)
1. **SINPE Móvil / IBAN reales** — hoy simulado. Requiere ser entidad supervisada **SUGEF** o ir patrocinado por un banco/fintech. Es la función estrella.
2. **Factura electrónica de Hacienda** (comprobante electrónico v4.4) para comercios.
3. **Liquidación real de servicios** (integración con cada biller).
4. **Remesas** baratas desde EE.UU.
5. **Custodia cripto on-chain real** (wallets reales, no ledger interno).

---

## ⚙️ Infra / calidad
- ✅ ~~**Tests automatizados**~~ — Go: `currency_test.go`, `i18n_test.go`, `hardening_test.go`, `recovery_test.go`, `totp_test.go` (lógica pura, sin DB; `go test ./...`). Front: `vitest` (`npm test`, `format.test.ts`). *Falta: tests de handlers con DB (necesitarían Postgres local o testcontainers).*
- ✅ ~~**Logging estructurado**~~ — `logging.go`: middleware `slogRequests` (JSON por request: método, ruta, status, duración, IP, request id; nivel según status) + `api.Logger` (slog) en `main.go`. *Falta: métricas y alertas.*
- **KYC real** (validación contra TSE / Registro Nacional; hoy auto-aprueba el formato).
- **Rate-limiting distribuido** (Upstash) si se escala a >1 instancia (hoy es en memoria, ok para 1 instancia de Render free).

---

## 🧭 Notas de arquitectura (para retomar rápido)
- **Backend** `backend/internal/api/`: handlers por dominio (`handlers.go`, `auth_handlers.go`, `sinpe.go`, `requests_handlers.go`, `pools_handlers.go`, `billers.go`, `webauthn.go`, `exchange.go`, `kyc_handlers.go`). Rutas en `server.go`. Hardening en `hardening.go`. i18n de errores en `i18n.go` (cabecera `X-Lang`).
- **Migraciones**: SQL numerado en `backend/internal/db/migrations/` (embebidas, corren con `RUN_MIGRATIONS=true`). Última: `0009_totp.sql`. `transactions.kind` es texto libre (`transfer|conversion|request|pool|service|sinpe`).
- **Catálogo de monedas**: `internal/api/currency.go` (backend) espejado en `src/currencies.ts` (front). Montos en unidades menores enteras por moneda (`toMinor`/`majorOf`).
- **i18n front**: `src/i18n.tsx` (claves ES/EN + selector). El cliente manda `X-Lang`.
- **Go 1.25** requerido (go-webauthn) → Dockerfile usa `golang:1.25-alpine`. Build local: Go portable en `$env:TEMP\goportable\go`; front `npm run build`. (Docker Desktop local crashea por un bug suyo — no se usa.)

## ▶️ Cómo retomar
Abrir Claude Code en `C:\Users\Keilor Martinez\Downloads\ticopay` y decir:
> "Continuá Tico Pay desde el ROADMAP.md — arrancá con [ítem]."
