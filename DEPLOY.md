# Deploy de Tico Pay

Pila gratis sin tarjeta: **Vercel** (frontend) + **Render** (backend Docker) + **Neon** (Postgres).
Flujo: `git push` a `main` → auto-deploy en Render y Vercel.

| Capa     | Proveedor | Plan            | Root directory |
| -------- | --------- | --------------- | -------------- |
| Frontend | Vercel    | Hobby           | `frontend`     |
| Backend  | Render    | Free Web (Docker) | `backend`    |
| Postgres | Neon      | Free            | —              |

> Tico Pay no usa Redis — Postgres alcanza. Si luego agregás rate-limiting/lockout, sumás Upstash.

## 1. Neon (Postgres)

1. Crear proyecto **ticopay**, Postgres 16.
2. Copiar la connection string:
   `postgresql://user:pass@ep-xxx.neon.tech/ticopay?sslmode=require`

## 2. Render (backend Go)

**New → Web Service** → repo de ticopay → Root Directory `backend`, Runtime **Docker**, Branch `main`, Instance **Free**.

Variables de entorno (coinciden con `backend/internal/config/config.go`):

| Variable         | Valor                                                  |
| ---------------- | ------------------------------------------------------ |
| `PORT`           | `8080`                                                 |
| `DATABASE_URL`   | la connection string de Neon                           |
| `JWT_SECRET`     | `openssl rand -base64 48` (≥32 chars)                  |
| `CORS_ORIGINS`   | `https://<tu-app>.vercel.app` (sin slash final)        |
| `RUN_MIGRATIONS` | `true` (solo 1er deploy / cuando agregués migración)   |
| `SEED_DEMO`      | `true` (solo 1er deploy)                               |

- Verificar: `https://ticopay.onrender.com/health`
- Tras el 1er deploy: **quitar** `RUN_MIGRATIONS` y `SEED_DEMO`.

> `CORS_ORIGINS` se carga como un slice de **un solo origen** en `config.go`. Si necesitás varios,
> hay que cambiar el parseo a split por comas.

## 3. Vercel (frontend)

**Add New → Project** → mismo repo → Framework **Vite**, Root Directory `frontend`.

| Variable       | Valor                              |
| -------------- | ---------------------------------- |
| `VITE_API_URL` | `https://ticopay.onrender.com` (sin slash final) |

## Notas

- El primer request a Render Free puede tardar ~50s (cold start del plan gratis).
- `DATABASE_URL` de Neon **requiere** `sslmode=require`; el pool de pgx lo respeta automáticamente.
