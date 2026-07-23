# TuanisPay — Roadmap

Estado y pendientes de TuanisPay (pagos CR full-stack). Pensado para retomar en una sesión nueva.

- **Frontend:** https://tuanispay.vercel.app · **API:** https://tuanispay.onrender.com
- **Stack:** Go (chi + pgx + JWT) en Render · React + Vite (TS) en Vercel · Postgres en Neon
- **Deploy:** `git push origin main` → Render y Vercel auto-despliegan (repo **público** `kryrmz/tuanispay`)
- **Cuenta demo:** `maria@tuanispay.cr` / `password123`

---

## ✅ Hecho y desplegado (verificado)
Auth (clave + **passkeys/WebAuthn** passwordless + **códigos de recuperación** de un solo uso) · multimoneda (₡, $, €, MXN + 15 cripto con precios CoinGecko + tipo de cambio BCCR) · **enviar** por teléfono/correo · **SINPE Móvil** simulado (con comprobante) · **convertir** entre cualquier par · **cobros** (con QR/WhatsApp) · **vaquitas** · **pago de servicios** (ICE, AyA, marchamo, RTV, CCSS…) · **KYC** cédula/DIMEX · patrimonio estimado · UI amigable con pestañas · **i18n ES/EN** (UI + errores del backend) · **rate-limiting + bloqueo de cuenta** (5 intentos → 15 min).

> ⚠️ **Importante:** cripto, servicios y SINPE son un **libro contable interno (simulado)** — no mueven plata real ni liquidan con blockchain/ICE/INS/SINPE real.

---

## 🟣 Riel de comercio (implementado y **desplegado**)
Cobro por QR de comercio inspirado en el modelo KiramoPay, montado sobre el wallet existente. Integrado a `main` y desplegado (Render + Vercel auto-despliegan); la integración del camino del dinero corre en CI contra un servicio Postgres.

- **Ledger de doble entrada** (`ledger_entries` + trigger DEFERRED de balanceo) y cuenta `SYSTEM:FEES`. Los saldos siguen siendo la fuente operativa; el ledger es el registro auditable de pagos wallet-to-wallet. Migración `0012`.
- **Idempotencia extremo a extremo** (`Idempotency-Key` + tabla `idempotency_keys`, migración `0013`) en enviar, SINPE, servicios, **convertir** y aportes; los cobros son idempotentes por `paid_by`. Cierra la **carrera de doble pago** que existía en cobros/vaquitas (ahora `FOR UPDATE` + una sola transacción).
- **Comercios** (`merchants`, migración `0014`): multi-comercio, KYC ligero, estados `pending/verified/rejected` y `commission_bps` (default 50 = 0,50 %). Solo un comercio verificado y propio cobra; la comisión es entera (`A*bps/10000`) y se asienta como `pagador −A, comercio +(A−f), SYSTEM:FEES +f`.
- **Rol admin server-side** (`users.role`, migración `0015`; `/api/admin/*`): el rol no viaja en el JWT ni en `/me`. Bootstrap en prod por env `ADMIN_EMAIL`; el seed demo deja admin a `maria@tuanispay.cr`. **Evolucionado a RBAC completo** — ver la sección "Back-office" más abajo.
- **Frontend:** secciones Comercio y Admin (i18n ES/EN), rótulo "simulado" en SINPE/Servicios, y sin reintento ciego de POSTs de dinero.
- **Ledger completo (migración `0016`):** las conversiones se asientan como FX balanceado contra `SYSTEM:FX` (usuario −F/+T, mesa FX +F/−T) y los pagos de servicios contra `SYSTEM:CLEARING`, así que **todo cambio de saldo es reconciliable** contra el ledger. La regla de saldo no-negativo pasó de un `CHECK` de columna a un trigger que exime a las cuentas de sistema (sus posiciones pueden ser negativas).
- **`Idempotency-Key` obligatoria** en los POST de dinero (send, SINPE, servicios, convertir, aportes): el server devuelve 400 si falta.
- **Pruebas:** integración del camino del dinero (`money_db_test.go`) **gateadas por `TEST_DATABASE_URL`** (corren en CI con servicio Postgres; se saltan en local). Nuevo `.github/workflows/ci.yml` (Go build/vet/test + frontend).

---

## 🟣 Back-office: roles, usuarios y reportes (implementado y **desplegado**)
Gestión de roles "digna de fintech" y panel de analítica, sobre el RBAC del riel de comercio.

- **RBAC de roles fijos** (`permissions.go`): `user`, `merchant`, `support`, `analyst`, `admin`, con una **matriz de permisos** (`backoffice.access`, `reports.view`, `users.manage`, `merchants.verify`, `merchants.commission`). `requirePerm` reemplaza a `requireAdmin` y **lee el rol de la BD en cada request** (nunca del cliente ni del JWT). `support` opera/verifica pero no toca comisiones ni usuarios; `analyst` es solo lectura.
- **Gestión de staff** (`staff.go`): crear usuarios con rol, cambiar rol, activar/desactivar. Desactivar **revoca las sesiones** (bump de `token_version`) y `requireAuth` bloquea cuentas desactivadas (`users.disabled`, migración `0017`). Guards: no quitarte tu propio admin, no dejar la plataforma sin administradores (advisory lock), no tocar los system users.
- **Panel de reportes** (`reports.go`, capacidad `reports.view`): overview de KPIs (usuarios/activos, comercios, transacciones, volumen y comisiones por moneda), series de tiempo, desglose por tipo, **salud del ledger** (neto por moneda = 0 + posiciones de sistema), y **transacciones filtrables** (fecha/tipo/moneda/texto, paginadas) con **export CSV** sanitizado contra inyección de fórmulas.
- **Frontend** (`sections/admin/`): panel con sub-pestañas Reportes / Usuarios / Comercios / Auditoría gateadas por **capacidades** (no por el rol; el server gatea cada endpoint), gráficos SVG modernos sin dependencias (`components/Charts.tsx`: área, barras, donut, KPIs), tabla filtrable + descarga CSV. i18n ES/EN.
- **Bitácora de auditoría** (`admin_audit_log`, migración `0018`): registro append-only de cada mutación del back-office (verificar/rechazar/comisión de comercios, alta/rol/estado de usuarios), escrito en la **misma transacción** que la mutación para que nunca se desfase. Lectura en `GET /api/admin/audit` (capacidad `reports.view`, paginado y filtrable por acción) + panel "Auditoría" (`sections/admin/Auditoria.tsx`).

> Revisión adversarial: corregidos inyección de fórmulas CSV, lockout del último admin (con advisory lock), bypass del self-guard por UUID en mayúsculas, errores silenciados en métricas y validación de fechas.

---

## 🟡 Pendientes — Seguridad / producción

### 1. ~~Recuperar contraseña~~ ✅ **Hecho y desplegado** *(falta solo `RESEND_API_KEY` en Render para que mande correos)*
- Migración `0010` (`password_reset_tokens`, token aleatorio 32 bytes, **solo hash SHA-256**, expira 30 min, un solo uso atómico). `POST /api/auth/forgot` (anti-enumeración: 200 constante + envío async fuera del request; invalida tokens previos) y `POST /api/auth/reset` (consume token, cambia hash, **bumpea `token_version`** → revoca todas las sesiones). Front: "¿Olvidaste tu contraseña?" en `AuthPage.tsx` + página `/reset`.
- **Capa de email** `internal/email/` enchufable: Resend (`RESEND_API_KEY`/`RESEND_FROM`) o fallback dev que loguea (solo imprime el enlace con `EMAIL_DEBUG=true`).

### 2. ~~Códigos de recuperación de passkey~~ ✅ **Hecho, desplegado y verificado en prod**
- 10 códigos de un solo uso (formato `XXXX-XXXX`, alfabeto sin glifos ambiguos), hasheados con bcrypt en `passkey_recovery_codes` (migración `0007`). Se muestran UNA sola vez; regenerar invalida los anteriores.
- Backend: `recovery.go` → `GET/POST /api/passkeys/recovery-codes` (autenticado) y `POST /api/auth/recovery` (login con código, comparte el bloqueo por intentos de `hardening.go`, key `recovery:<email>`). Tests en `recovery_test.go`.
- Front: `sections/Account.tsx` (sección "🛟 Códigos de recuperación": estado/generar/regenerar/copiar) y `pages/AuthPage.tsx` (enlace "¿Perdiste tu llave?" → entrar con código). i18n ES/EN agregado.
- Verificado E2E contra prod: generar → entrar con código → el código se consume (reuso da 401, `remaining` baja). ✓

### 3. ~~Verificación de correo al registrarse~~ ✅ **Hecho y desplegado** *(usa la misma capa de email)*
- Columna `users.email_verified` + `email_verification_tokens` (migración `0010`). El registro manda el correo (async, best-effort). `POST /api/auth/verify-email` (consumo atómico de token) y `POST /api/auth/verify-email/send` (reenvío, autenticado). Front: banner en el Dashboard + página `/verify-email`. Usuarios existentes y demo quedan verificados (backfill + seed).
- **Endurecimiento extra del review de seguridad**: revocación de sesión por `token_version` (migración `0011`, validada en `requireAuth`/`refresh`), cierre del timing-oracle de login (bcrypt dummy), `CORS_ORIGINS` como lista separada por comas.
### 4. ~~2FA TOTP como alternativa a passkeys~~ ✅ **Hecho**
- Migración `0009_totp.sql` (tabla `user_totp`: secreto por usuario, gate solo si `confirmed`). Backend `totp.go`: `GET /api/totp` (estado), `POST /api/totp/setup` (secreto + otpauth URL), `/confirm` (valida 1er código y activa), `/disable` (pide código válido). Login: con 2FA activo responde **428** si falta `totpCode`; código malo cuenta para el lockout.
- Front: sección "📱 Verificación en dos pasos" en `Account.tsx` (QR con `qrcode.react` + clave manual + confirmar/desactivar); `AuthPage.tsx` muestra campo de código al recibir 428. i18n ES/EN.

### 5. ~~Endurecimiento adversarial (back-office y sesiones)~~ ✅ **Hecho y desplegado** *(tras una auditoría adversarial)*
- **Arranque seguro en prod:** con `APP_ENV=production` el server aborta si `JWT_SECRET` está vacío/por defecto/`<32` (fail-closed), no siembra la demo, fija cabeceras de seguridad (`secureHeaders`: CSP deny-all, anti-framing, nosniff, Referrer-Policy, HSTS), limita el body a 1 MiB (`maxBody`) y aplica un tope de 120 req/min por IP en rutas autenticadas. Nuevos `config.AppEnv`/`Config.IsProd()`.
- **Bitácora de auditoría** (`admin_audit_log`, migración `0018`): ver la sección Back-office. Registro append-only y transaccional de toda mutación privilegiada, con lectura en `GET /api/admin/audit` y panel "Auditoría".
- **Sesiones:** `POST /api/auth/logout` autenticado que bumpea `token_version` (salir de **todos** los dispositivos); `/auth/refresh` rechaza cuentas desactivadas (defensa en profundidad); `RefreshTTL` reducido de 7 días a **48 h**.
- **Credenciales:** contraseña de personal ≥10 con letras y dígitos (`validateStaffPassword`); anti-enumeración en el inicio de login por passkey (misma respuesta exista o no la cuenta); **anti-replay TOTP** (migración `0019`, `user_totp.last_used_period`): se registra el periodo de 30 s consumido y se rechaza el reuso del mismo código en login/confirmación/desactivación.
- **Pendiente (parte 2):** mover el refresh a **cookie httpOnly + CSRF** y sacar el access token de `localStorage`. Bloqueado por la topología **cross-site** actual (`tuanispay.vercel.app` ↔ `tuanispay.onrender.com`): una cookie de refresh tendría que ser de terceros (`SameSite=None`), que Safari bloquea y Chrome retira. Hacerlo bien exige un despliegue same-site (dominio propio `app.tuanispay.cr` + `api.tuanispay.cr`, o un proxy de Vercel `/api/*`→Render) para usar cookie first-party `SameSite=Lax`.

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
- ✅ ~~**Tests automatizados**~~ — Go: `currency_test.go`, `i18n_test.go`, `hardening_test.go`, `recovery_test.go`, `totp_test.go`, `password_test.go` (lógica pura, sin DB; `go test ./...`). Tests de handlers con DB (`money_db_test.go`, `merchant_db_test.go`, `staff_db_test.go`) gateados por `TEST_DATABASE_URL`: corren en CI contra un servicio Postgres, se saltan en local. Front: `vitest` (`npm test`, `format.test.ts`).
- ✅ ~~**Logging estructurado**~~ — `logging.go`: middleware `slogRequests` (JSON por request: método, ruta, status, duración, IP, request id; nivel según status) + `api.Logger` (slog) en `main.go`. *Falta: métricas y alertas.*
- **KYC real** (validación contra TSE / Registro Nacional; hoy auto-aprueba el formato).
- **Rate-limiting distribuido** (Upstash) si se escala a >1 instancia (hoy es en memoria, ok para 1 instancia de Render free).

---

## 🧭 Notas de arquitectura (para retomar rápido)
- **Backend** `backend/internal/api/`: handlers por dominio (`handlers.go`, `auth_handlers.go`, `sinpe.go`, `requests_handlers.go`, `pools_handlers.go`, `billers.go`, `webauthn.go`, `exchange.go`, `kyc_handlers.go`). Rutas en `server.go`. Hardening en `hardening.go`. i18n de errores en `i18n.go` (cabecera `X-Lang`).
- **Migraciones**: SQL numerado en `backend/internal/db/migrations/` (embebidas, corren con `RUN_MIGRATIONS=true`). Última: `0019_totp_replay.sql`. `transactions.kind` es texto libre (`transfer|conversion|request|merchant|pool|service|sinpe`).
- **Catálogo de monedas**: `internal/api/currency.go` (backend) espejado en `src/currencies.ts` (front). Montos en unidades menores enteras por moneda (`toMinor`/`majorOf`).
- **i18n front**: `src/i18n.tsx` (claves ES/EN + selector). El cliente manda `X-Lang`.
- **Go 1.25** requerido (go-webauthn) → Dockerfile usa `golang:1.25-alpine`. Build local: Go portable en `$env:TEMP\goportable\go`; front `npm run build`. (Docker Desktop local crashea por un bug suyo — no se usa.)

## ▶️ Cómo retomar
Abrir Claude Code en `C:\Users\Keilor Martinez\Downloads\tuanispay` y decir:
> "Continuá TuanisPay desde el ROADMAP.md — arrancá con [ítem]."
