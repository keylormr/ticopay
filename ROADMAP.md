# Tico Pay — Roadmap

Estado y pendientes de Tico Pay (pagos CR full-stack). Pensado para retomar en una sesión nueva.

- **Frontend:** https://ticopay.vercel.app · **API:** https://ticopay.onrender.com
- **Stack:** Go (chi + pgx + JWT) en Render · React + Vite (TS) en Vercel · Postgres en Neon
- **Deploy:** `git push origin main` → Render y Vercel auto-despliegan (repo **público** `kryrmz/ticopay`)
- **Cuenta demo:** `maria@ticopay.cr` / `password123`

---

## ✅ Hecho y desplegado (verificado)
Auth (clave + **passkeys/WebAuthn** passwordless) · multimoneda (₡, $ + 15 cripto con precios CoinGecko + tipo de cambio BCCR) · **enviar** por teléfono/correo · **SINPE Móvil** simulado (con comprobante) · **convertir** entre cualquier par · **cobros** (con QR/WhatsApp) · **vaquitas** · **pago de servicios** (ICE, AyA, marchamo, RTV, CCSS…) · **KYC** cédula/DIMEX · patrimonio estimado · UI amigable con pestañas · **i18n ES/EN** (UI + errores del backend) · **rate-limiting + bloqueo de cuenta** (5 intentos → 15 min).

> ⚠️ **Importante:** cripto, servicios y SINPE son un **libro contable interno (simulado)** — no mueven plata real ni liquidan con blockchain/ICE/INS/SINPE real.

---

## 🟡 Pendientes — Seguridad / producción

### 1. Recuperar contraseña  *(necesita proveedor de correo)*
- **Bloqueador:** entregar el enlace/código requiere email (Resend o SendGrid, plan gratis → API key en env de Render).
- **Approach:** migración `password_reset_tokens` (token hasheado + expiración ~30 min); `POST /api/auth/forgot` (genera token, manda email) y `POST /api/auth/reset` (valida token, cambia hash). Front: enlace "¿Olvidaste tu contraseña?" en `AuthPage.tsx`.

### 2. Códigos de recuperación de passkey  *(autocontenido, se puede ya)*
- Generar 8–10 códigos de un solo uso, hashear con bcrypt, guardar en tabla `passkey_recovery_codes`. Mostrarlos UNA vez al crear el primer passkey.
- `POST /api/auth/recovery` (login con un código → consume el código, emite tokens). Front: en `Account.tsx` (mostrar/regenerar) y `AuthPage.tsx` (entrar con código).

### 3. Verificación de correo al registrarse  *(necesita correo, igual que #1)*
### 4. (Opcional) 2FA TOTP como alternativa a passkeys (`github.com/pquerna/otp`).

---

## 🔵 Pendientes — Pulido
- **Nombre personalizado** del passkey al registrar (hoy se guarda fijo "Mi dispositivo"); editar en `sections/Account.tsx` + `handlePasskeyRegisterFinish`.
- Traducir los pocos **errores 500 técnicos** restantes (mapa en `internal/api/i18n.go`).
- **Quitar `RUN_MIGRATIONS` y `SEED_DEMO`** de las env vars de Render (ya corrieron; son idempotentes).
- **Más fiat** (EUR, MXN, …): necesita feed FX gratis (p. ej. `frankfurter.app`). Agregar al catálogo `internal/api/currency.go` + `src/currencies.ts` y al cálculo de `usdPerUnit` en `internal/api/exchange.go`.

---

## 🟢 Roadmap mayor (lo que lo haría imbatible en CR)
1. **SINPE Móvil / IBAN reales** — hoy simulado. Requiere ser entidad supervisada **SUGEF** o ir patrocinado por un banco/fintech. Es la función estrella.
2. **Factura electrónica de Hacienda** (comprobante electrónico v4.4) para comercios.
3. **Liquidación real de servicios** (integración con cada biller).
4. **Remesas** baratas desde EE.UU.
5. **Custodia cripto on-chain real** (wallets reales, no ledger interno).

---

## ⚙️ Infra / calidad
- **Tests automatizados** (Go `testing`/`httptest`; front `vitest`).
- **Observabilidad**: logging estructurado, métricas, alertas.
- **KYC real** (validación contra TSE / Registro Nacional; hoy auto-aprueba el formato).
- **Rate-limiting distribuido** (Upstash) si se escala a >1 instancia (hoy es en memoria, ok para 1 instancia de Render free).

---

## 🧭 Notas de arquitectura (para retomar rápido)
- **Backend** `backend/internal/api/`: handlers por dominio (`handlers.go`, `auth_handlers.go`, `sinpe.go`, `requests_handlers.go`, `pools_handlers.go`, `billers.go`, `webauthn.go`, `exchange.go`, `kyc_handlers.go`). Rutas en `server.go`. Hardening en `hardening.go`. i18n de errores en `i18n.go` (cabecera `X-Lang`).
- **Migraciones**: SQL numerado en `backend/internal/db/migrations/` (embebidas, corren con `RUN_MIGRATIONS=true`). Última: `0006_webauthn_flags.sql`. `transactions.kind` es texto libre (`transfer|conversion|request|pool|service|sinpe`).
- **Catálogo de monedas**: `internal/api/currency.go` (backend) espejado en `src/currencies.ts` (front). Montos en unidades menores enteras por moneda (`toMinor`/`majorOf`).
- **i18n front**: `src/i18n.tsx` (claves ES/EN + selector). El cliente manda `X-Lang`.
- **Go 1.25** requerido (go-webauthn) → Dockerfile usa `golang:1.25-alpine`. Build local: Go portable en `$env:TEMP\goportable\go`; front `npm run build`. (Docker Desktop local crashea por un bug suyo — no se usa.)

## ▶️ Cómo retomar
Abrir Claude Code en `C:\Users\Keilor Martinez\Downloads\ticopay` y decir:
> "Continuá Tico Pay desde el ROADMAP.md — arrancá con [ítem]."
