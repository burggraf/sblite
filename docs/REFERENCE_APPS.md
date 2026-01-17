# Reference Applications for Testing sblite

This document lists open-source Supabase applications that can be used to test sblite's API compatibility. These are more substantial than simple todo apps and exercise real-world usage patterns.

## Compatibility Legend

| Symbol | Meaning |
|--------|---------|
| ✅ | Supported by sblite |
| ⚠️ | Uses feature not yet implemented |
| ❌ | Not used by app |

---

## Recommended Applications

### 1. FinOpenPOS ⭐ **Best Fit**

**Repository**: https://github.com/JoaoHenriqueBarbosa/FinOpenPOS

A Point of Sale and Inventory Management System built with Next.js, React, and Supabase.

**Tech Stack:**
- Next.js + React
- Tailwind CSS + Shadcn UI
- Supabase (PostgreSQL)
- Recharts for visualization

**Database Schema:**
| Table | Purpose |
|-------|---------|
| Products | Inventory data |
| Customers | User information |
| Orders | Transaction records |
| Order Items | Line items |
| Transactions | Financial records |
| Payment Methods | Payment configuration |

**Supabase Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ✅ | ✅ |
| REST API (CRUD) | ✅ Heavy | ✅ |
| Realtime | ❌ | ⚠️ Not implemented |
| Storage | ❌ | ⚠️ Not implemented |
| Edge Functions | ❌ | ⚠️ Not implemented |

**Why it's good**: Complex multi-table relationships, heavy REST API usage, real-world POS use case. No dependencies on unimplemented features.

---

### 2. CMSaasStarter ⭐ **Best for Auth Testing**

**Repository**: https://github.com/CriticalMoments/CMSaasStarter

A modern SaaS template built with SvelteKit, Tailwind, and Supabase.

**Tech Stack:**
- SvelteKit
- Tailwind CSS
- Supabase
- Stripe (external, for payments)
- Cloudflare Workers (optional)

**Features:**
- Marketing pages
- Blog system
- User dashboard
- User settings
- Pricing page
- Subscription management

**Auth Features Exercised:**
| Feature | sblite Support |
|---------|----------------|
| Email/password signup | ✅ |
| Email/password login | ✅ |
| Password recovery | ⚠️ Not implemented |
| Email verification | ⚠️ Not implemented |
| OAuth (GitHub, Google) | ⚠️ Not implemented |
| User profiles | ✅ |

**Supabase Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ✅ Heavy | ✅ Partial |
| REST API | ✅ | ✅ |
| Realtime | ❌ | ⚠️ Not implemented |
| Storage | ❌ | ⚠️ Not implemented |
| Edge Functions | ❌ | ⚠️ Not implemented |

**Why it's good**: Comprehensive auth flows, uses Stripe externally (not Supabase edge functions). Good for stress-testing `/auth/v1/*` endpoints.

---

### 3. Atomic CRM

**Repository**: https://github.com/marmelab/atomic-crm

A full-featured CRM built with React, react-admin, shadcn/ui, and Supabase.

**Tech Stack:**
- React + react-admin
- shadcn/ui
- Supabase
- TypeScript

**Features:**
| Feature | Description |
|---------|-------------|
| Contacts | Organization and management |
| Deals | Kanban board visualization |
| Tasks | Creation with reminders |
| Notes | Activity history tracking |
| Import/Export | CSV data handling |
| Custom Fields | Extensible schema |

**Auth Providers:**
- Google
- Azure
- Keycloak
- Auth0

**Supabase Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ✅ | ✅ (email/password only) |
| REST API | ✅ Heavy | ✅ |
| Realtime | ❌ | ⚠️ Not implemented |
| Storage | ⚠️ Yes (attachments) | ⚠️ Not implemented |
| Edge Functions | ❌ | ⚠️ Not implemented |

**Compatibility Notes**: Uses Supabase Storage for file attachments. Would need to disable/mock that feature or implement storage support.

---

### 4. svelte-kanban (Official Community)

**Repository**: https://github.com/supabase-community/svelte-kanban

A Trello clone from the official Supabase community.

**Tech Stack:**
- Svelte
- PLpgSQL (82% of codebase)
- Supabase

**Features:**
- Boards management
- Lists within boards
- Cards/tasks within lists
- Drag and drop

**Supabase Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ❓ Unknown | ✅ |
| REST API | ✅ | ✅ |
| Realtime | ❌ | ⚠️ Not implemented |
| Storage | ❌ | ⚠️ Not implemented |
| Edge Functions | ❌ | ⚠️ Not implemented |

**Why it's good**: Simple, official example. Includes `setup.sql` for schema. Database-driven architecture.

---

### 5. Basejump (Advanced - Multi-Tenant)

**Repository**: https://github.com/usebasejump/basejump

Multi-tenant SaaS foundation with teams, invitations, and roles.

**Tech Stack:**
- PLpgSQL (98% of codebase)
- React components
- Supabase

**Features:**
| Feature | Description |
|---------|-------------|
| Personal Accounts | Auto-created on signup |
| Team Accounts | Shared, billable accounts |
| Invitations | Team member invites |
| Roles | Permission management |
| Billing | Stripe integration |

**Supabase Features Used:**
| Feature | Used | sblite Support |
|---------|------|----------------|
| Auth | ✅ | ✅ |
| REST API | ✅ | ✅ |
| RLS Policies | ⚠️ Heavy | ⚠️ Not implemented |
| Realtime | ❌ | ⚠️ Not implemented |
| Storage | ❌ | ⚠️ Not implemented |
| Edge Functions | ⚠️ (billing) | ⚠️ Not implemented |

**Compatibility Notes**: Heavy use of Row Level Security policies. Would require RLS implementation to work properly.

---

## Quick Comparison Matrix

| App | Auth | REST CRUD | Realtime | Storage | Edge Fn | RLS | Complexity |
|-----|------|-----------|----------|---------|---------|-----|------------|
| **FinOpenPOS** | ✅ | ✅ Heavy | ❌ | ❌ | ❌ | ❓ | High |
| **CMSaasStarter** | ✅ Heavy | ✅ | ❌ | ❌ | ❌ | ❓ | Medium |
| **Atomic CRM** | ✅ | ✅ Heavy | ❌ | ⚠️ | ❌ | ❓ | High |
| **svelte-kanban** | ❓ | ✅ | ❌ | ❌ | ❌ | ❓ | Low |
| **Basejump** | ✅ | ✅ | ❌ | ❌ | ⚠️ | ⚠️ Heavy | High |

---

## Implementation Priority

### Phase 1: Basic Compatibility
1. **FinOpenPOS** - Test complex CRUD operations and multi-table queries
2. **svelte-kanban** - Simple database-driven app, good baseline

### Phase 2: Auth Enhancements
3. **CMSaasStarter** - Requires password recovery, email verification

### Phase 3: Advanced Features
4. **Atomic CRM** - Requires Storage implementation
5. **Basejump** - Requires RLS implementation

---

## Setup Notes

### Pointing Apps to sblite

Most Supabase apps use environment variables:

```bash
# Instead of Supabase cloud
SUPABASE_URL=http://localhost:8080
SUPABASE_ANON_KEY=<your-jwt-secret>

# Or for Next.js apps
NEXT_PUBLIC_SUPABASE_URL=http://localhost:8080
NEXT_PUBLIC_SUPABASE_ANON_KEY=<your-jwt-secret>
```

### Schema Migration

Each app will have its own schema requirements. Check for:
- `setup.sql` or `schema.sql` files
- `supabase/migrations/` directory
- Prisma schema files (`prisma/schema.prisma`)

Run these against sblite's SQLite database, adapting PostgreSQL-specific syntax as needed.

---

## Resources

- [Supabase Official Examples](https://supabase.com/docs/guides/resources/examples)
- [awesome-supabase](https://github.com/lyqht/awesome-supabase)
- [Supabase GitHub Examples](https://github.com/supabase/supabase/tree/master/examples)
