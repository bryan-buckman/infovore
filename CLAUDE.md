# Infovore - Project Instructions

## Project Overview

Infovore is a self-hosted Go 1.22 monolith combining an **RSS reader** and a **Kalshi prediction market scanner** in a single tabbed web UI. PostgreSQL is the only supported database (SQLite was removed).

## Tech Stack

| Component   | Technology                                          |
| ----------- | --------------------------------------------------- |
| Language    | Go 1.22                                             |
| HTTP Router | chi/v5                                              |
| Database    | PostgreSQL via lib/pq (Store interface in `internal/database/store.go`) |
| RSS Parsing | gofeed                                              |
| Frontend    | Vanilla HTML/CSS/JS, Go html/template (embedded via go:embed) |
| Container   | Multi-stage Docker (golang:1.22 -> debian:bookworm-slim) |
| Deployment  | Docker -> FluxCD -> Kubernetes on Proxmox VMs       |

## Read on Session Start

Before making changes, read:
- `docs/architecture.md` — package layout and key design decisions
- `Dockerfile` — current deployment config

## Key Architecture

- **Single binary**: all templates, CSS, JS embedded at compile time
- **Database**: `internal/database/store.go` defines the `Store` interface (30+ methods); sole implementation is `PostgresStore` in `postgres.go`
- **RSS fetching**: parallel worker pool (8 workers), domain-level rate limiting (2 concurrent per domain + 500ms delay)
- **Kalshi scanner**: background goroutine checks every 5 min if a scan is due; RSA-PSS auth, 20 req/sec rate limit
- **Config precedence**: CLI flags > environment variables > `.env` file (at project root or `/data/.env`)

## Deployment Context

- In production, `DB_URL` comes from a Kubernetes Secret mounted at `/data/.env` — there are no CLI flags in prod
- PostgreSQL 15+ requires `GRANT CREATE ON SCHEMA public` for the app user (common gotcha)
- App connection pool: 25 max open connections
- The `.env` file is in `.gitignore`; sensitive config never goes in the repo

## After Code Changes Checklist

1. Verify the `Dockerfile` still accurately reflects the build (user has asked this across multiple sessions)
2. Verify `.env` precedence is still respected (env vars set before process start must not be overridden by `loadEnvFile`)
3. RSS domain-level rate limiting must remain intact — do not remove or weaken it
4. Run: `go vet ./... && go build ./...`

## User Preferences

- Gives feature requests in numbered batches (3-5 at a time) — work through them sequentially
- Has intermediate Go experience — do not start tutorials at beginner level
- Prefers analysis before changes (e.g., connection count estimates, VM sizing, performance comparisons)
- This app runs locally/internally — security is not a major concern but don't introduce obvious vulnerabilities
- When the user says "review this codebase", read the claude.md files and key source files before responding

## Documentation

Detailed docs live in `docs/`:
- `architecture.md` — system architecture and package layout
- `api.md` — full HTTP API reference
- `data-flows.md` — mermaid sequence diagrams for all major flows
- `db.md` — schema, Kalshi settings, and Proxmox VM sizing recommendations
- `getting-started.md` — setup, usage, Docker, and Kubernetes deployment

---

# FASTER Learning System - Instructions

## System Overview

This project uses the FASTER framework:

-   **F**orget: Beginner's mindset
-   **A**ct: Hands-on practice
-   **S**tate: Optimize focus
-   **T**each: Explain to retain
-   **E**nter: Consistency over intensity
-   **R**eview: Spaced repetition (1d → 3d → 7d → 14d → 30d → 60d → 90d)

## Directory Structure

```
project-root/
├── CLAUDE.md (this file)
├── .claude/
│   ├── agents/practice-creator.md
│   ├── commands/
│   │   ├── learn.md
│   │   ├── review.md
│   │   └── progress.md
│   └── settings.local.json
└── .learning/
    ├── scripts/
    │   ├── init_learning.py
    │   ├── log_progress.py
    │   ├── review_scheduler.py
    │   └── generate_syllabus.py
    ├── references/
    │   └── faster_framework.md
    └── <topic-slug>/
        ├── metadata.json
        ├── syllabus.md
        ├── progress.md
        ├── review_schedule.json
        └── mastery.md
```

## Session Protocol

### EVERY Session Start

The system automatically:

1. Checks for due reviews (via context gathering in commands)
2. Conducts reviews BEFORE new learning if any are due
3. Guides you through the session flow

### Session Flow

```
START
  ↓
[1] Check reviews → Conduct if due
  ↓
[2] State check: "Are you focused?"
  ↓
[3] Present next syllabus item
  ↓
[4] User learns/builds/practices
  ↓
[5] Ask: "Explain it back to me"
  ↓
[6] Log progress → Add to review schedule
  ↓
[7] Remind: "Next session: [time]"
  ↓
END
```

## Script Usage

All scripts are in `.learning/scripts/`. Run from project root.

### Initialize Topic

**User action:** `/learn "Topic Name"`

**Flow:**

```bash
python3 .learning/scripts/init_learning.py "<Topic Name>" .learning
```

→ **Action:** Create comprehensive syllabus tailored to user's level and focus

### Log Progress

```bash
python3 .learning/scripts/log_progress.py <topic-slug> "<summary>" [concept1] [concept2]
```

→ **Action:** Add each concept to review schedule

### Review Management

```bash
# Check status
python3 .learning/scripts/review_scheduler.py status <topic-slug>

# Add concept
python3 .learning/scripts/review_scheduler.py add <topic-slug> "<Concept>"

# Mark reviewed
python3 .learning/scripts/review_scheduler.py review <topic-slug> "<Concept>"
```

### Topic Info

```bash
# List all topics
python3 .learning/scripts/generate_syllabus.py list

# Get topic details
python3 .learning/scripts/generate_syllabus.py info <topic-slug>
```

## Execution Rules

**✅ ALWAYS:**

1. Check reviews at session start
2. Parse JSON output from scripts
3. Follow `next_action` and `llm_directive` fields
4. Prompt user to teach concepts back
5. Log every learning activity
6. Add learned concepts to review schedule
7. Generate comprehensive syllabi (not minimal)

**❌ NEVER:**

1. Skip review checks
2. Let user passively consume
3. Forget to log progress
4. Skip adding concepts to reviews
5. Generate minimal syllabi

## Workflow Pattern

```
[RUN SCRIPT] → [EXECUTE DIRECTIVE] → [RESPOND TO USER]
```

## Generating Syllabus

When `next_action: "generate_syllabus"`:

1. **Read** `.learning/<topic-slug>/syllabus.md` (created by init script)
2. **Replace placeholder** with comprehensive syllabus tailored to user's level and focus
3. **Include sections**: Overview, Prerequisites, Learning Objectives, 3-4 Phases with 🔨 hands-on projects, Teaching Milestones, Resources, Success Criteria
4. **Update metadata**: Set `"syllabus_generated": true` in `.learning/<topic-slug>/metadata.json`

## Teaching Prompts

After learning concepts, use `AskUserQuestion` to prompt teach-back:

```json
{
    "question": "Ready to teach back what you just learned?",
    "header": "Teach Back",
    "multiSelect": false,
    "options": [
        {
            "label": "Yes, let me explain",
            "description": "I'll explain the concept in my own words"
        },
        {
            "label": "Need review first",
            "description": "Want to review the concept again"
        },
        {
            "label": "Not sure yet",
            "description": "Need more practice before explaining"
        }
    ]
}
```

If user chooses "Yes, let me explain":

-   "Explain [concept] in your own words"
-   "How would you teach this to a beginner?"
-   "What analogy would you use?"

## Progress Tracking

**Milestones:**

-   Every 5 sessions: Show progress report
-   Weekly: Full review of trajectory
-   When stuck: Review learned concepts, identify gaps

**Check session count:**

```bash
cat .learning/<topic-slug>/metadata.json | grep total_sessions
```

**Recent progress:**

```bash
tail -30 .learning/<topic-slug>/progress.md
```

## Key Principles for This System

-   Use `AskUserQuestion` to gather learning preferences
-   Always prompt user to teach concepts back

**For User:**

-   1 project = 1 learning goal
-   30min daily > 3hr weekly (consistency over intensity)
-   Active learning > passive consumption
-   Teaching = best retention
-   Trust the spaced repetition system
