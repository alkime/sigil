# Insights Engine — Architecture Design

## Context

Penny Helps needs a system to proactively surface relevant information to Trust Circles — scam alerts, local activities, financial summaries, health updates. The **Insights Engine** is the backbone that connects external data sources, processes them through LLM-powered analysis, and delivers curated reports/alerts to the home page.

**First phase**: Web-based insights via Firecrawl (scam awareness + local activities).
**Future phases**: Plaid (financial), Nylas (email), HealthEx (health).

This document covers the full architecture, but only Phase 1 will be broken into implementation tasks.

---

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Trust Circle                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Firecrawl │  │ Firecrawl │  │  Plaid   │  (future)   │
│  │ Scams-PHX │  │ Events-SC │  │ Checking │              │
│  └─────┬─────┘  └─────┬─────┘  └─────┬────┘             │
│        │              │              │                    │
└────────┼──────────────┼──────────────┼────────────────────┘
         │              │              │
         ▼              ▼              ▼
   ┌─────────────────────────────────────────┐
   │           River Queue (Postgres)         │
   │                                          │
   │  ┌─────────┐  ┌──────────┐  ┌────────┐ │
   │  │  Sync   │  │ LLM      │  │Publish │ │
   │  │  Jobs   │  │ Analysis │  │ Jobs   │ │
   │  └─────────┘  └──────────┘  └────────┘ │
   └─────────────────────────────────────────┘
         │              │              │
         ▼              ▼              ▼
   ┌─────────────────────────────────────────┐
   │              Database (Ent)              │
   │                                          │
   │  Integration → SyncRun → Insight         │
   │                                          │
   └─────────────────────────────────────────┘
         │
         ▼
   ┌─────────────────────────────────────────┐
   │         Home Page (React)                │
   │  Published alerts & reports per circle   │
   └─────────────────────────────────────────┘
```

---

## 2. Data Model

### 2.1 Integration

Represents a configured connection between a Trust Circle and an external data source.

```
<!-- @review-ref 0001 -->
Integration
  id              uuid (PK)
  trust_circle_id uuid (FK → TrustCircle, nullable for global)
  source_type     enum: firecrawl, plaid, nylas, healthex
  name            string       — user-friendly label, e.g. "Local Scams - Phoenix"
  status          enum: active, paused, error, setup
  settings        jsonb        — source-specific config (see below)
  credentials     jsonb        — encrypted OAuth2 tokens (nullable, not for Firecrawl)
  schedule        string       — cron expression, default "0 2 * * *" (2am daily)
  last_synced_at  timestamptz  — nullable
  error_message   string       — nullable, last error if status=error
  created_by_id   uuid (FK → UserProfile)
  created_at      timestamptz
  updated_at      timestamptz
```

**Settings by source_type:**

```jsonc
// Firecrawl - scam awareness
{
  "category": "scam_awareness",
  "location": { "city": "Phoenix", "state": "AZ", "zip": "85001" },
  "target_urls": ["https://local-news.example.com/scam-alerts"],
  "search_queries": ["elder scams Phoenix AZ 2026"],
  "max_pages_per_sync": 20
}

// Firecrawl - local activities
{
  "category": "local_activities",
  "location": { "city": "Scottsdale", "state": "AZ", "zip": "85251" },
  "target_urls": ["https://scottsdale.gov/events"],
  "search_queries": ["senior activities Scottsdale AZ"],
  "max_pages_per_sync": 20
}

// Plaid (future)
{
  "institution_name": "Chase",
  "account_mask": "1234",
  "sync_transactions": true,
  "alert_thresholds": { "large_withdrawal": 500 }
}
```

**Key design decisions:**
- `trust_circle_id` nullable → supports global insights (platform-wide scam alerts)
- Multiple integrations per circle to same source_type → different locations, accounts
- `status: paused` → sync jobs check this and skip; no schedule modification needed
- `credentials` separate from `settings` → different access control, encryption at rest
- `schedule` as cron string → directly compatible with River's periodic job system. River uses `robfig/cron` for [complex cron schedules](https://riverqueue.com/docs/periodic-jobs#complex-cron-schedules), and its `Schedule` interface is identical to River's `PeriodicSchedule`. On startup, we query active integrations and register each via `river.NewPeriodicJob(cron.ParseStandard(integration.Schedule), ...)`. At runtime, `Client.PeriodicJobs().Add()`/`.Remove()` handles create/pause/resume. The DB column is the durable record; River's in-memory schedule is the runtime mechanism.

**Multi-instance safety:** River uses [Postgres-based leader election](https://riverqueue.com/docs/periodic-jobs) so that only one instance in the cluster actually enqueues periodic jobs, even when multiple backend instances register the same schedules. During leader failover, a job could theoretically be skipped if the transition lands exactly on the cron trigger — low risk for nightly syncs, but we can mitigate with River's [unique jobs](https://riverqueue.com/docs/unique-jobs) + `RunOnStart` option to guarantee at-most-once execution per period.

### 2.2 SyncRun

Tracks each execution of an integration's sync job.

```
SyncRun
  id              uuid (PK)
  integration_id  uuid (FK → Integration)
  status          enum: running, completed, failed, cancelled
  started_at      timestamptz
  completed_at    timestamptz (nullable)
  items_fetched   int         — raw items retrieved from source
  insights_created int        — insights generated by LLM
  error_message   string      — nullable
  metadata        jsonb       — source-specific run details
```

### 2.3 Insight

<!-- @review-ref 0002 -->
The core output — an LLM-generated report or alert derived from source data.

```
Insight
  id              string (ULID, PK) — time-ordered like Messages
  integration_id  uuid (FK → Integration)
  trust_circle_id uuid (FK → TrustCircle, nullable for global)
  sync_run_id     uuid (FK → SyncRun)

  category        enum: scam_alert, local_activity, financial_alert,
                        health_update, general
  severity        enum: info, warning, critical

  title           string
  summary         string       — 1-2 sentence preview for cards
  content         text         — full markdown content
  source_urls     text[]       — original URLs that informed this insight
  source_data     jsonb        — raw extracted data (for audit/debugging)

  status          enum: draft, review, published, dismissed, archived
  published_at    timestamptz  (nullable)
  reviewed_by_id  uuid (FK → UserProfile, nullable)
  reviewed_at     timestamptz  (nullable)
  expires_at      timestamptz  (nullable) — for time-sensitive content

  content_hash    string       — dedup: hash of key content to avoid duplicates
  notification_sent bool       — false; set true when chat/push notification delivered
  llm_model       string       — nullable; model that generated this insight (e.g. "claude-sonnet-4-5-20250929")

  created_at      timestamptz
  updated_at      timestamptz
```

**Status flow:**
```
draft → review → published → archived
                → dismissed
```

- `draft` — LLM just generated it, may need refinement
- `review` — ready for superadmin review (default landing state)
- `published` — visible to the Trust Circle on home page
- `dismissed` — superadmin reviewed and rejected (keep for analytics)
- `archived` — was published, now expired or manually archived

**Key design decisions:**
- ULID for natural time ordering (same pattern as Message)
- `content_hash` prevents duplicate insights across sync runs (same scam article scraped twice)
- `source_data` preserves raw input for debugging LLM outputs
- `trust_circle_id` denormalized from Integration for efficient queries (avoids join for home page)
- `expires_at` for time-sensitive content (events with dates, limited-time scam campaigns)

### 2.4 Entity Relationships

```
TrustCircle
  └── integrations[]     (one-to-many)
        └── sync_runs[]  (one-to-many)
        └── insights[]   (one-to-many)

UserProfile
  └── integrations_created[]  (one-to-many, created_by)
  └── insights_reviewed[]     (one-to-many, reviewed_by)
```

<!-- @review-ref 0003 -->
### 2.5 Ent Schema Edges (New + Modified)

This section specifies the Ent ORM edge definitions needed for the new entities. Edges are how Ent models foreign key relationships — each edge becomes a queryable traversal in generated code (e.g., `integration.QuerySyncRuns()`). Existing schemas (TrustCircle, UserProfile) get new edges added; new schemas (Integration, SyncRun, Insight) are defined entirely here.

**Existing schemas — new edges to add:**

| Schema | New Edge | Type | Target | Purpose |
|--------|----------|------|--------|---------|
| TrustCircle | `integrations` | O2M | Integration | A circle owns its configured integrations |
| UserProfile | `integrations_created` | O2M | Integration | Track who set up each integration |
| UserProfile | `insights_reviewed` | O2M | Insight | Track which admin reviewed each insight |

**New schemas — full edge definitions:**

| Schema | Edge | Type | Target | Notes |
|--------|------|------|--------|-------|
| Integration | `trust_circle` | M2O | TrustCircle | Optional (nullable for global integrations) |
| Integration | `created_by` | M2O | UserProfile | Required — who configured this |
| Integration | `sync_runs` | O2M | SyncRun | History of all sync executions |
| Integration | `insights` | O2M | Insight | All insights produced by this integration |
| SyncRun | `integration` | M2O | Integration | Required — which integration ran |
| Insight | `integration` | M2O | Integration | Required — which integration produced this |
| Insight | `trust_circle` | M2O | TrustCircle | Optional (nullable for global); denormalized from Integration for query performance |
| Insight | `sync_run` | M2O | SyncRun | Required — which run produced this |
| Insight | `reviewed_by` | M2O | UserProfile | Optional — set when admin publishes/dismisses |

---

## 3. River Queue — Background Job System

### 3.1 Why River

- **Postgres-native** — uses our existing Supabase Postgres, no new infrastructure
- **Go-native** — first-class Go library, fits our stack
- **Reliable** — exactly-once processing, automatic retries, dead letter queue
- **Periodic jobs** — built-in cron scheduling, no external cron needed
- **Transactional** — jobs can be enqueued within Ent transactions

### 3.2 Job Types

| Job | Trigger | Purpose |
|-----|---------|---------|
| `SyncIntegration` | Cron (per integration schedule) | Fetch data from source via Firecrawl/Plaid/etc |
| `AnalyzeContent` | Enqueued by SyncIntegration | Send fetched content to LLM for insight generation |
| `ExpireInsights` | Daily cron | Archive insights past their `expires_at` |

**Flow for a single sync cycle:**

```
1. River cron fires SyncIntegration{integration_id}
2. Worker checks integration.status — skip if paused
3. Worker creates SyncRun (status: running)
4. Worker calls Firecrawl API with integration.settings
5. For each page of content:
   a. Check content_hash — skip if insight already exists
   b. Enqueue AnalyzeContent{sync_run_id, raw_content, source_url}
6. Worker updates SyncRun (status: completed, items_fetched: N)

7. AnalyzeContent worker picks up job
8. Calls Claude API with category-specific prompt + raw content
9. Creates Insight (status: review) with LLM-generated title/summary/content
10. Updates SyncRun.insights_created counter
```

### 3.3 Integration with Server

<!-- @review-ref 0004 -->
River client starts alongside the Gin server in `cmd/server/main.go`:

```go
// Pseudocode for server startup
riverClient := river.NewClient(pgxPool, riverConfig)

// Register workers
river.AddWorker(riverClient, &SyncIntegrationWorker{...})
<!-- @review-ref 0005 -->
river.AddWorker(riverClient, &AnalyzeContentWorker{...})

// Start periodic jobs from active integrations
// (or use River's periodic job feature with cron expressions)
riverClient.Start(ctx)
```

Workers live in `api/internal/jobs/` (new package).

### 3.4 Package Structure

```
api/internal/jobs/
  ├── jobs.go              — River client setup, worker registration
  ├── sync_integration.go  — SyncIntegration worker
  ├── analyze_content.go   — AnalyzeContent worker (LLM calls)
  └── expire_insights.go   — ExpireInsights worker
```

---

## 4. Firecrawl Integration (Phase 1)

### 4.1 What Firecrawl Does

Firecrawl is an API service that:
- Scrapes web pages and returns clean, structured content (markdown/text)
- Handles JavaScript rendering, anti-bot measures
- Supports search queries (returns relevant URLs)
- Respects robots.txt
- Has a Go SDK or simple REST API

### 4.2 Sync Flow

```
SyncIntegration worker:
  1. Read integration.settings (category, location, target_urls, search_queries)
  2. For each target_url:
     - Call Firecrawl scrape API → get clean markdown content
  3. For each search_query:
     - Call Firecrawl search API → get relevant URLs
     - Scrape top N results
  4. Deduplicate by URL (skip already-processed URLs via content_hash)
  5. Enqueue AnalyzeContent for each new piece of content
```

### 4.3 LLM Analysis Prompts

<!-- @review-ref 0006 -->
Two prompt templates by category:

**Scam Awareness:**
```
You are analyzing web content for scam alerts relevant to elderly adults
in {location}. Extract any scam warnings, fraud alerts, or suspicious
activity reports. For each finding, produce:
- A clear, non-alarming title
- A 1-2 sentence summary suitable for a home page card
- Detailed content in markdown with: what the scam is, how to recognize it,
  what to do if contacted
- Severity: critical (active/local threat), warning (emerging pattern),
  info (general awareness)

If the content contains no relevant scam information, return null.
```

**Local Activities:**
```
You are analyzing web content for activities, events, groups, and social
opportunities relevant to elderly adults in {location}. Extract events and
activities that would help combat isolation and keep elders engaged. For each:
- An inviting, clear title
- A 1-2 sentence summary with date/location if available
- Detailed content with: what it is, when/where, how to participate,
  accessibility info if available
- Severity: always "info"

If the content contains no relevant activities, return null.
```

### 4.4 Firecrawl Configuration

```go
// api/internal/platform/config/config.go additions
type FirecrawlConfig struct {
    APIKey         string `env:"FIRECRAWL_API_KEY"`
    BaseURL        string `env:"FIRECRAWL_BASE_URL" envDefault:"https://api.firecrawl.dev"`
    MaxPagesPerJob int    `env:"FIRECRAWL_MAX_PAGES" envDefault:"20"`
}
```

---

## 5. Curation & Moderation

### 5.1 Review Workflow

All LLM-generated insights land in `status: review`. Superadmins access a review queue:

1. **Review Queue** — list insights in `review` status, newest first
2. **Review Action** — for each insight:
   - **Publish** → `status: published`, sets `published_at`, `reviewed_by_id`
   - **Dismiss** → `status: dismissed`, sets `reviewed_by_id` (with optional reason)
   - **Edit & Publish** → modify title/summary/content, then publish
3. **Bulk actions** — publish/dismiss multiple insights at once

### 5.2 Global vs Circle Insights

| Type | trust_circle_id | Visibility | Example |
|------|----------------|------------|---------|
| Global | NULL | All circles (once published) | "FTC warns of new Medicare scam" |
| Circle-local | set | Only that circle | "Phoenix Senior Center: Free Tax Help Event" |

Global insights come from superadmin-managed global integrations — same Integration model, just no `trust_circle_id`.

### 5.3 Future: Notifications

When a critical/warning insight is published, the system should eventually:
1. **Penny posts in circle chat** — a system message from Penny linking to the insight
2. **Push notifications** — for critical severity (future, requires push infra)

The `notification_sent` flag on Insight tracks whether delivery has happened, preventing duplicate notifications if an insight is re-published after editing. This is deferred from Phase 1 but the schema is ready.

### 5.4 Future: Auto-publish

The schema supports `draft → review → published` but nothing prevents adding:
- Confidence scoring from LLM
- Auto-publish for high-confidence + info severity
- Auto-publish for trusted source URLs (allowlist)

This is intentionally deferred — start manual, build trust in the system.

---

## 6. API Design (Connect-RPC)

### 6.1 InsightsService

```proto
service InsightsService {
  // Home page — published insights for the user's circle
  rpc ListInsights(ListInsightsRequest) returns (ListInsightsResponse);

  // Single insight detail view
  rpc GetInsight(GetInsightRequest) returns (GetInsightResponse);

  // Integration management (steward+)
  rpc ListIntegrations(ListIntegrationsRequest) returns (ListIntegrationsResponse);
  rpc CreateIntegration(CreateIntegrationRequest) returns (CreateIntegrationResponse);
  rpc UpdateIntegration(UpdateIntegrationRequest) returns (UpdateIntegrationResponse);
  rpc PauseIntegration(PauseIntegrationRequest) returns (PauseIntegrationResponse);
  rpc ResumeIntegration(ResumeIntegrationRequest) returns (ResumeIntegrationResponse);
  rpc DeleteIntegration(DeleteIntegrationRequest) returns (DeleteIntegrationResponse);

  // Sync history
  rpc ListSyncRuns(ListSyncRunsRequest) returns (ListSyncRunsResponse);
  rpc TriggerSync(TriggerSyncRequest) returns (TriggerSyncResponse);  // Manual sync
}
```

### 6.2 InsightsAdminService

```proto
service InsightsAdminService {
  // Review queue
  rpc ListInsightsForReview(ListInsightsForReviewRequest) returns (ListInsightsForReviewResponse);
  rpc PublishInsight(PublishInsightRequest) returns (PublishInsightResponse);
  rpc DismissInsight(DismissInsightRequest) returns (DismissInsightResponse);
  rpc BulkPublishInsights(BulkPublishInsightsRequest) returns (BulkPublishInsightsResponse);

  // Global integration management
  rpc CreateGlobalIntegration(CreateGlobalIntegrationRequest) returns (CreateGlobalIntegrationResponse);
}
```

### 6.3 Key Query Patterns

**Home page feed** (most critical query):
```sql
SELECT * FROM insights
WHERE (trust_circle_id = $1 OR trust_circle_id IS NULL)  -- circle + global
  AND status = 'published'
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY severity DESC, created_at DESC
LIMIT 20;
```

**Review queue:**
```sql
SELECT * FROM insights
WHERE status = 'review'
ORDER BY created_at DESC;
```

---

## 7. Frontend — Home Page

### 7.1 Home Page Redesign

The current home page (`clients/web/src/routes/index.tsx`) is a placeholder. It becomes the **Insights Feed**:

```
┌─────────────────────────────────────┐
│  Hi Alice, I'm Penny                │
│                                      │
│  ┌─ CRITICAL ──────────────────────┐ │
│  │  Medicare Scam Alert            │ │
│  │  FTC reports surge in calls...  │ │
│  │  [Read More]                    │ │
│  └─────────────────────────────────┘ │
│                                      │
│  ┌─ WARNING ───────────────────────┐ │
│  │  Local: Package Delivery Scam   │ │
│  │  Phoenix PD reports increase... │ │
│  │  [Read More]                    │ │
│  └─────────────────────────────────┘ │
│                                      │
│  ── Activities Near You ──           │
│  ┌──────────┐ ┌──────────┐          │
│  │ Free Tax  │ │ Walking  │          │
│  │ Help      │ │ Group    │          │
│  │ Mar 28    │ │ Weekly   │          │
│  └──────────┘ └──────────┘          │
│                                      │
└─────────────────────────────────────┘
```

- Alerts (critical/warning) shown prominently at top
- Activities shown as cards in a grid
- "Read More" opens a detail view (could be a drawer or new route)
- Grouped by category: scam alerts first, then activities

### 7.2 Admin Review Page

New route: `/admin/insights` — the review queue for superadmins.

---

## 8. Package & File Structure (New)

```
api/
  schema/
    integration.go          — Ent schema
    syncrun.go              — Ent schema
    insight.go              — Ent schema
  internal/
    biz/
      insights.go           — InsightsService business logic
      insights_admin.go     — Admin review operations
    handlers/
      insights.go           — Connect-RPC InsightsService handler
      insights_admin.go     — Connect-RPC InsightsAdminService handler
    jobs/
      jobs.go               — River client setup
      sync_integration.go   — Sync worker
      analyze_content.go    — LLM analysis worker
      expire_insights.go    — Expiry worker
    platform/
      config/
        config.go           — + FirecrawlConfig, RiverConfig, AnthropicConfig
      llm/
        llm.go              — Claude API client wrapper
        prompts.go          — Prompt templates by category

proto/
  services/v1/
    insights.proto          — InsightsService + messages
    insights_admin.proto    — InsightsAdminService
  ent/v1/
    insight.proto           — Insight entity proto
    integration.proto       — Integration entity proto

clients/web/src/
  routes/
    index.tsx               — Redesigned home page with insights feed
    admin/
      insights.tsx          — Review queue page
  components/
    insights/
      InsightCard.tsx        — Card component for feed
      InsightDetail.tsx      — Full insight view
      InsightReviewCard.tsx  — Admin review card with actions
```

---

## 9. Dependencies (New)

| Package | Purpose |
|---------|---------|
| `github.com/riverqueue/river` | Postgres-backed job queue |
| `github.com/riverqueue/river/riverdriver/riverpgxv5` | River driver for pgx v5 |
| `github.com/anthropics/anthropic-sdk-go` | Claude API for LLM analysis |
| Firecrawl Go SDK or REST client | Web scraping API |

---

## 10. Phase Breakdown

### Phase 1: Foundation + Web Insights (implement now)
1. River Queue setup (infrastructure)
2. Ent schemas (Integration, SyncRun, Insight)
3. Firecrawl integration (sync worker)
4. LLM analysis worker (Claude API)
5. Insights API (list/get for home page)
6. Admin review API + frontend
7. Home page redesign

### Phase 2: Financial Insights (future)
- Plaid OAuth2 flow
- Transaction sync worker
- Financial alert prompts (unusual spending, large withdrawals)

### Phase 3: Email Insights (future)
- Nylas OAuth2 flow
- Email scanning worker
- Suspicious email detection prompts

### Phase 4: Health Insights (future)
- HealthEx integration
- Health data sync
- Health trend analysis prompts

---

## 11. Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Location config | Integration-level setting | Most flexible — multiple locations per circle via separate integrations |
| Moderation | Superadmin-only (for now) | Tight control while tuning LLM quality |
| Notifications | Design for chat + push, defer implementation | Schema has `notification_sent` flag; Penny chat delivery is Phase 1.5 |
| Global insights | Superadmin-managed global integrations | Same Integration model, `trust_circle_id = NULL` |

## 12. Open Questions (Implementation-Time)

1. **Firecrawl pricing/limits** — need to understand rate limits to set appropriate `max_pages_per_sync` defaults
2. **LLM cost per sync** — each scraped page = 1 Claude API call; need to estimate monthly cost per circle
3. **Content retention** — how long to keep dismissed/archived insights and raw source_data?

<!--
@review-backmatter

"0001":
  offset: 1
  span: 15
  comment: "Does the cron expression in schedule match up with how riverqueue jobs are scheduled?\n\nREPLY: Yes — River uses robfig/cron which accepts standard cron expressions directly via its PeriodicSchedule interface. Added clarification on how the DB column maps to runtime registration, and a note on multi-instance safety via River's Postgres-based leader election (only the leader enqueues periodic jobs, even across multiple load-balanced instances)."
  status: resolved

"0002":
  offset: 1
  span: 37
  comment: "Let's add an `llm_model_that_authored` string which is optional and can just hole the name of the LLM model that generated it.\n\nREPLY: Added llm_model (nullable string) to the Insight schema — records which model generated the insight, useful for comparing output quality across model versions."
  status: resolved

"0003":
  offset: 1
  span: 1
  comment: "Can you expand on this section a bit more I'm not totally sure what it's trying to express?\n\nREPLY: Expanded into two tables — one for new edges on existing schemas (TrustCircle, UserProfile), one for full edge definitions on new schemas. Added explanatory intro about what Ent edges are and why they matter."
  status: resolved

"0004":
  offset: 1
  span: 1
  comment: "should we account for cleanup of SyncRun jobs?"
  status: open

"0005":
  offset: 1
  span: 1
  comment: "Is there a Stop associated with the client like Start? We need some way to add graceful shutdown for things the server does. If there isn’t, maybe we could have a simple RiverService wrapper that waits for all workers to finish?\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b\u200b"
  status: open

"0006":
  offset: 1
  span: 59
  comment: "One thing to not is the go sdk is \"community\" based -- not official, but seems like is in a recent version."
  status: open

-->
