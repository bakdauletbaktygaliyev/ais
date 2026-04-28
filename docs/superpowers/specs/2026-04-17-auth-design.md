# Auth Design — AIS

**Date:** 2026-04-17  
**Status:** Approved

## Summary

Add email+password authentication with JWT tokens. Each user sees only their own projects (full isolation by `user_id`). Token stored in `localStorage` on the frontend.

---

## 1. Database

New table:

```sql
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMP DEFAULT NOW()
);
```

Add column to existing table:

```sql
ALTER TABLE projects ADD COLUMN user_id UUID REFERENCES users(id);
```

All project queries filter by `user_id` — users only see their own projects.

---

## 2. Backend API

### New public endpoints

| Method | Path | Body | Response |
|--------|------|------|----------|
| POST | `/api/auth/register` | `{ email, password }` | `{ token, user_id }` |
| POST | `/api/auth/login` | `{ email, password }` | `{ token, user_id }` |

### Protected endpoints (require `Authorization: Bearer <token>`)

```
POST   /api/analyze
GET    /api/projects
GET    /api/projects/:id
GET    /api/projects/:id/graph
DELETE /api/projects/:id
```

### New files

- `internal/handler/auth.go` — register and login handlers
- `internal/middleware/auth.go` — JWT middleware, injects `user_id` into Gin context
- `internal/db/users.go` — `CreateUser`, `GetUserByEmail`

### Dependencies

- `github.com/golang-jwt/jwt/v5` — JWT signing/verification
- `golang.org/x/crypto/bcrypt` — password hashing

### Token

- Lifetime: 24 hours
- Signed with `JWT_SECRET` env var

---

## 3. Frontend (Angular 17)

### New pages

- `pages/auth/login/` — login form
- `pages/auth/register/` — registration form

### New service

- `services/auth.service.ts` — stores JWT in `localStorage`, exposes `login()`, `register()`, `logout()`, `isLoggedIn()`

### New guard

- `guards/auth.guard.ts` — redirects to `/login` if no valid token

### New interceptor

- `interceptors/auth.interceptor.ts` — automatically attaches `Authorization: Bearer <token>` to all API requests

### Routes

```
/login       → LoginComponent     (public)
/register    → RegisterComponent  (public)
/            → HomeComponent      (guarded)
/project/:id → ProjectComponent   (guarded)
```

### Changes to existing files

- `app.routes.ts` — add `/login`, `/register`; apply `authGuard` to `/` and `/project/:id`
- `app.config.ts` — register HTTP interceptor

---

## 4. Validation and Error Handling

### Backend

- Password minimum 8 characters (server-side check)
- Duplicate email → `409 Conflict`
- Wrong credentials → `401 Unauthorized` (same message for both cases — do not reveal whether email exists)
- Invalid or expired JWT → `401 Unauthorized`
- Access to another user's project → `404 Not Found` (not 403 — do not reveal existence)

### Frontend

- Inline field errors (invalid email format, short password)
- On `401` from API → automatic logout and redirect to `/login`
- Logout button in header (clears `localStorage`)
