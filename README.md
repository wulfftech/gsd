<div align="center">
  <img src="ui/public/android-chrome-192x192.png" alt="GSD" width="80"/>
  <h1>GSD — Gettin' Shit Done</h1>
  <p><strong>Self-hosted household task & chore management for the whole family.</strong></p>
</div>

---

GSD is a self-hosted fork of [Donetick](https://github.com/donetick/donetick), extended with a suite of household-focused features built on top of the solid upstream foundation.

## What's in the box

### Core (upstream Donetick)
- **Recurring tasks** with flexible scheduling — daily, weekly, monthly, adaptive, and more
- **Natural language task creation** — "Take the trash out every Monday at 6pm"
- **Subtasks** with smart reset on completion
- **Assignee rotation** — round-robin, random, or least-completed
- **Time tracking** per task session
- **Labels, priorities, filters**
- **Circle** — share tasks across a household or team
- **Notifications** — Telegram, Pushover, Discord, FCM push

### Added in this fork
- **Projects** — collections of tasks with assignees, due dates, and a points reward. Any assignee can mark a task done; admins approve. Completing all tasks awards points to every assignee.
- **Rewards** — circle members redeem accumulated points for admin-defined rewards.
- **Dashboard** — at-a-glance view of tasks due in the next 24h alongside a per-member team card showing assigned tasks, points balance, and completions today.
- **Managed accounts** — any circle admin can create, rename, reset passwords for, and delete lightweight login accounts (no email required). No longer tied to a single owner.
- **Missed chore auto-advance** — overdue recurring chores are automatically marked Missed and their due date advanced so the list stays clean.
- **Admin approval badge** — admins see a live "N to approve" chip in the header when project tasks are waiting.

## Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go · Gin · GORM · SQLite |
| Frontend | React · Vite · MUI Joy UI · TanStack Query |
| Container | Docker · single-stage multi-build Dockerfile |

## Quick start

```bash
git clone https://github.com/wulfftech/gsd.git
cd gsd
docker compose up -d
```

Then open **http://localhost:2021** in your browser.

### Configuration

Copy `config/selfhosted.yaml` and adjust as needed. **Important:** the shipped `jwt.secret` value (`"change_this_to_a_secure_random_string_32_characters_long"`) is a placeholder and must be replaced with a real random value before first start. Without this change, the container will crash on startup. Generate a secure secret with:

```bash
openssl rand -base64 32
```

Then paste the output into `config/selfhosted.yaml`'s `jwt.secret` field.

Key environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DT_ENV` | `selfhosted` | Config profile |
| `DT_SQLITE_PATH` | `/donetick-data/donetick.db` | Database path |
| `TZ` | — | Timezone (e.g. `Australia/Perth`) |

## Development

```bash
# Backend
go run .

# Frontend (separate terminal)
cd ui
npm install
npm run dev
```

## Deploying updates

```bash
git pull
docker compose build
docker compose up -d
```

---

*Forked from [donetick/donetick](https://github.com/donetick/donetick). Upstream licence applies — see [LICENSE.md](LICENSE.md).*
