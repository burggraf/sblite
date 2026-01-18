# Reference Applications for Testing sblite

This document lists open-source Supabase applications that can be used to test sblite's API compatibility. These range from simple examples to substantial real-world apps that exercise multiple features.

## Compatibility Legend

| Symbol | Meaning |
|--------|---------|
| ✅ | Supported by sblite |
| ⚠️ | Uses feature not yet implemented (Realtime only) |
| ❌ | Not used by app |

**Current sblite Feature Support:**
- ✅ Auth (email/password, anonymous, OAuth)
- ✅ REST API (full CRUD)
- ✅ Storage (local and S3 backends)
- ✅ Edge Functions (TypeScript/JavaScript)
- ✅ Row Level Security (RLS)
- ⚠️ Realtime (not yet implemented)

---

## Tier 1: Official Supabase Examples (Highest Priority)

These are maintained by Supabase and represent canonical patterns for using the platform.

### Todo List Apps

| App | GitHub | Features | Framework | Notes |
|-----|--------|----------|-----------|-------|
| Next.js Todo List | [supabase/.../nextjs-todo-list](https://github.com/supabase/supabase/tree/master/examples/todo-list/nextjs-todo-list) | Auth, Database, RLS | Next.js | Simple CRUD with Row Level Security |
| SvelteJS Todo List | [supabase/.../sveltejs-todo-list](https://github.com/supabase/supabase/tree/master/examples/todo-list/sveltejs-todo-list) | Auth, Database, RLS | Svelte | Same schema as Next.js version |

### User Management Examples

| App | GitHub | Features | Framework | Notes |
|-----|--------|----------|-----------|-------|
| Next.js User Management | [supabase/.../nextjs-ts-user-management](https://github.com/supabase/supabase/tree/master/examples/user-management/nextjs-ts-user-management) | Auth, Database | Next.js + TS | Profile management, avatars |
| React User Management | [supabase/.../react-user-management](https://github.com/supabase/supabase/tree/master/examples/user-management/react-user-management) | Auth, Database | React | Basic auth flows |
| Vue 3 User Management | [supabase/.../vue3-user-management](https://github.com/supabase/supabase/tree/master/examples/user-management/vue3-user-management) | Auth, Database | Vue 3 | Composition API |
| Svelte User Management | [supabase/.../svelte-user-management](https://github.com/supabase/supabase/tree/master/examples/user-management/svelte-user-management) | Auth, Database | Svelte | Minimal setup |
| SvelteKit User Management | [supabase/.../sveltekit-user-management](https://github.com/supabase/supabase/tree/master/examples/user-management/sveltekit-user-management) | Auth, Database | SvelteKit | SSR auth |

### Auth Examples

| App | GitHub | Features | Framework | Notes |
|-----|--------|----------|-----------|-------|
| Next.js Auth | [supabase/.../auth/nextjs](https://github.com/supabase/supabase/tree/master/examples/auth/nextjs) | Auth | Next.js | Email/password, OAuth flows |

### Storage Examples

| App | GitHub | Features | Framework | Notes |
|-----|--------|----------|-----------|-------|
| Storage Examples | [supabase/.../storage](https://github.com/supabase/supabase/tree/master/examples/storage) | Storage | Various | File upload/download patterns |

---

## Tier 2: Real-World Community Apps

These are substantial applications that exercise multiple Supabase features.

### FinOpenPOS ⭐ **Best Fit for CRUD Testing**

**Repository**: https://github.com/JoaoHenriqueBarbosa/FinOpenPOS

A Point of Sale and Inventory Management System built with Next.js, React, and Supabase.

**Tech Stack:** Next.js + React, Tailwind CSS + Shadcn UI, Supabase, Recharts

**Database Schema:**
| Table | Purpose |
|-------|---------|
| Products | Inventory data |
| Customers | User information |
| Orders | Transaction records |
| Order Items | Line items |
| Transactions | Financial records |
| Payment Methods | Payment configuration |

**Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ✅ | ✅ |
| REST API (CRUD) | ✅ Heavy | ✅ |
| Realtime | ❌ | ⚠️ Not implemented |
| Storage | ❌ | ✅ |

**Why it's good**: Complex multi-table relationships, heavy REST API usage, real-world POS use case.

---

### CMSaasStarter ⭐ **Best for Auth Testing**

**Repository**: https://github.com/CriticalMoments/CMSaasStarter

A modern SaaS template built with SvelteKit, Tailwind, and Supabase.

**Tech Stack:** SvelteKit, Tailwind CSS, Supabase, Stripe (external)

**Features:** Marketing pages, Blog system, User dashboard, Settings, Subscription management

**Auth Features Exercised:**
| Feature | sblite Support |
|---------|----------------|
| Email/password signup | ✅ |
| Email/password login | ✅ |
| Password recovery | ✅ |
| OAuth (GitHub, Google) | ✅ |
| User profiles | ✅ |

---

### Atomic CRM

**Repository**: https://github.com/marmelab/atomic-crm

A full-featured CRM built with React, react-admin, shadcn/ui, and Supabase.

**Features:** Contacts, Deals (Kanban), Tasks, Notes, Import/Export, Custom Fields

**Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ✅ | ✅ |
| REST API | ✅ Heavy | ✅ |
| Storage | ✅ (attachments) | ✅ |
| Realtime | ❌ | ⚠️ |

---

### svelte-kanban (Official Community)

**Repository**: https://github.com/supabase-community/svelte-kanban

A Trello clone from the official Supabase community.

**Tech Stack:** Svelte, PLpgSQL, Supabase

**Features:** Boards, Lists, Cards/tasks, Drag and drop

**Why it's good**: Simple, official example. Includes `setup.sql` for schema.

---

### Otter Bookmark Manager

**Repository**: https://github.com/mrmartineau/Otter

Self-hosted bookmark manager with Mastodon integration.

**Tech Stack:** Next.js (Cloudflare Workers SPA), TypeScript, Supabase

**Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ✅ | ✅ |
| Database | ✅ | ✅ |
| Realtime | ❌ | ⚠️ |

---

### Basejump (Advanced - Multi-Tenant)

**Repository**: https://github.com/usebasejump/basejump

Multi-tenant SaaS foundation with teams, invitations, and roles.

**Features:** Personal Accounts, Team Accounts, Invitations, Roles, Billing

**Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ✅ | ✅ |
| REST API | ✅ | ✅ |
| RLS Policies | ✅ Heavy | ✅ |
| Edge Functions | ✅ (billing) | ✅ |

---

## Tier 3: Simple Community Examples

Good for quick compatibility testing.

| App | GitHub | Features | Framework | Notes |
|-----|--------|----------|-----------|-------|
| supabase-react-crud-app | [Sayli29/supabase-react-crud-app](https://github.com/Sayli29/supabase-react-crud-app) | Database | React + Vite | Simple CRUD operations |
| supabase-todo-app | [keiloktql/supabase-todo-app](https://github.com/keiloktql/supabase-todo-app) | Auth, Database | Next.js | Todo with auth |
| supabase-todo-list | [vishnugopal/supabase-todo-list](https://github.com/vishnugopal/supabase-todo-list) | Auth, Database, RLS | - | RLS-based auth |
| SupaLink | [suciptoid/supalink](https://github.com/suciptoid/supalink) | Auth, Database | - | URL shortener |
| Calendry | [Huilensolis/calendry](https://github.com/Huilensolis/calendry) | Auth, Database | - | Open source calendar |
| Boonda | [get-boonda/boonda](https://github.com/get-boonda/boonda) | Auth, Storage, Database | - | Quick file sharing |

---

## Tier 4: Starter Templates

More complex, good for comprehensive feature testing.

| App | GitHub | Features | Framework | Notes |
|-----|--------|----------|-----------|-------|
| Nextbase Lite | [imbhargav5/nextbase-nextjs-supabase-starter](https://github.com/imbhargav5/nextbase-nextjs-supabase-starter) | Auth, Database | Next.js 16 | Full starter with TypeScript |
| next-supabase-starter | [Mohamed-4rarh/next-supabase-starter](https://github.com/Mohamed-4rarh/next-supabase-starter) | Auth, Database | Next.js 15 | React Query integration |
| Saas-Kit-supabase | [Saas-Starter-Kit/Saas-Kit-supabase](https://github.com/Saas-Starter-Kit/Saas-Kit-supabase) | Auth, Database, CRUD | Next.js | SaaS template |

---

## Apps to AVOID (Use Realtime)

These apps rely on Supabase Realtime and won't work with sblite until Realtime is implemented:

- **Slack Clone** (`supabase/supabase/.../slack-clone`) - Real-time messaging
- **Supabase Chat** - Real-time chat
- **SyncList** - Real-time syncing
- **Data Loom** - Real-time data sharing
- **Any multiplayer/collaboration app**

---

## Quick Comparison Matrix

| App | Auth | REST CRUD | Storage | Edge Fn | RLS | Realtime | Complexity |
|-----|------|-----------|---------|---------|-----|----------|------------|
| **Official Todo List** | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | Low |
| **User Management** | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | Low |
| **FinOpenPOS** | ✅ | ✅ Heavy | ❌ | ❌ | ❓ | ❌ | High |
| **CMSaasStarter** | ✅ Heavy | ✅ | ❌ | ❌ | ❓ | ❌ | Medium |
| **Atomic CRM** | ✅ | ✅ Heavy | ✅ | ❌ | ❓ | ❌ | High |
| **svelte-kanban** | ❓ | ✅ | ❌ | ❌ | ❓ | ❌ | Low |
| **Otter** | ✅ | ✅ | ❌ | ❌ | ❓ | ❌ | Medium |
| **Basejump** | ✅ | ✅ | ❌ | ✅ | ✅ Heavy | ❌ | High |

---

## Recommended Test Order

### Phase 1: Basic Compatibility
1. **Official Next.js Todo List** - Tests Auth, Database CRUD, RLS
2. **Official User Management** - Tests auth flows (signup, login, logout)

### Phase 2: Feature Validation
3. **Storage Examples** - Tests file upload, download, bucket operations
4. **supabase-react-crud-app** - Simple CRUD validation

### Phase 3: Real-World Apps
5. **FinOpenPOS** - Complex multi-table CRUD operations
6. **svelte-kanban** - Simple database-driven app
7. **Atomic CRM** - Tests Storage + complex queries

### Phase 4: Advanced Features
8. **CMSaasStarter** - Comprehensive auth flows
9. **Basejump** - Heavy RLS + Edge Functions

---

## Setup Notes

### Pointing Apps to sblite

Most Supabase apps use environment variables:

```bash
# Standard
SUPABASE_URL=http://localhost:8080
SUPABASE_ANON_KEY=<your-anon-key>
SUPABASE_SERVICE_ROLE_KEY=<your-service-role-key>

# Next.js apps
NEXT_PUBLIC_SUPABASE_URL=http://localhost:8080
NEXT_PUBLIC_SUPABASE_ANON_KEY=<your-anon-key>

# SvelteKit apps
PUBLIC_SUPABASE_URL=http://localhost:8080
PUBLIC_SUPABASE_ANON_KEY=<your-anon-key>
```

### Schema Migration

Each app will have its own schema requirements. Check for:
- `setup.sql` or `schema.sql` files
- `supabase/migrations/` directory
- Prisma schema files (`prisma/schema.prisma`)

Run these against sblite using `sblite db push` after copying to `./migrations/`.

### Potential Compatibility Notes

- **UUIDs**: sblite uses proper UUID v4 format (compatible)
- **Timestamps**: sblite stores as ISO 8601 strings
- **RLS policies**: sblite supports RLS via query rewriting
- **Storage**: sblite has Supabase-compatible Storage API (local and S3 backends)
- **Edge Functions**: sblite supports Deno-based edge functions
- **PostgreSQL-specific SQL**: May need adaptation for SQLite

---

## Resources

- [Supabase Official Examples](https://github.com/supabase/supabase/tree/master/examples)
- [Awesome Supabase](https://github.com/lyqht/awesome-supabase) - Official curated list
- [Made with Supabase](https://www.madewithsupabase.com/) - Community showcase
- [OpenAlternative Supabase Stack](https://openalternative.co/stacks/supabase) - Open source projects
