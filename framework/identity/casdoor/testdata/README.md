# Casdoor v4.1.0 fixtures

Sanitized wire samples captured from a local Casdoor instance so the adapter in
this package (Phase 2/4) and its tests pin real shapes instead of guessed ones.
Contract details live in `docs/architecture/identity/casdoor-bridge.mdx`.

| Field | Value |
|---|---|
| Casdoor version | `v4.1.0` |
| Commit | `9603cddddb84b06fb1db917c91ea1b8e60c5a7d6` (`/api/get-version-info`) |
| Captured | 2026-09-16 |
| Organization | `echojoy` (users/groups), actor `built-in/admin` |

## Files

| File | Source request | What it pins |
|---|---|---|
| `discovery.json` | `GET /.well-known/openid-configuration` | issuer, endpoints, `S256`, `offline_access`, RS256 |
| `user.json` | `GET /api/get-user?id=echojoy/<name>` | full `User` object; `id` is a UUID; `externalId` holds a third-party-login value (`<provider>:<uuid>`), not a bridge key |
| `group.json` | `GET /api/get-group?id=echojoy/dev-group-1` | `Group` object; `users: null`; `updatedTime` carries a `+08:00` offset |
| `get-users-page.json` | `GET /api/get-users?owner=echojoy&pageSize=2&p=1&sortField=updatedTime&sortOrder=descend` | paging envelope; `data2` is the exact total (3090) |
| `get-user-missing.json` | `GET /api/get-user?userId=<uuid-of-deleted-user>&owner=echojoy` | `{"status":"ok","data":null}` is the hard-delete signal |
| `webhook-add-user.json` | webhook delivery for `add-user` | `Record` + `extendedUser`; `object` has `owner`/`name` but **no `id`** |
| `webhook-update-user.json` | webhook delivery for `update-user` | `object` has `id` (UUID) of the affected user |
| `webhook-delete-user.json` | webhook delivery for `delete-user` | same shape as update |
| `webhook-add-group.json` | webhook delivery for `add-group` | group `object` (no UUID) |
| `webhook-update-group.json` | webhook delivery for `update-group` | same |
| `webhook-delete-group.json` | webhook delivery for `delete-group` | same |

Every webhook sample shows that `Record.user`, `Record.organization` and
`extendedUser` describe the **actor** (`built-in/admin`), while the affected
user or group is the JSON string in `Record.object`. Deliveries were made with
`isUserExtended: true` and a static `Authorization: Bearer <secret>` header.

## Capture procedure

1. `POST /api/login` with the admin account to obtain the session cookie.
2. Fetch discovery, `get-version-info`, `get-users`, `get-groups`, `get-user`.
3. Start a local HTTP listener that stores POST bodies; register a webhook
   (`POST /api/add-webhook`) on the **actor's** organization (`built-in`) with
   `isUserExtended: true` and all six user/group events.
4. Create a temporary user (in `echojoy/dev-group-1`) and a temporary group,
   update both, delete both. Because the Casdoor delivery worker only starts
   when a webhook already exists at boot, the queued events stayed `Pending`;
   they were delivered with `POST /api/replay-webhook-event?id=<owner/name>`.
5. Query `get-user?userId=<deleted uuid>&owner=echojoy` for the missing sample.
6. Delete the webhook and its events (`delete-webhook`, `delete-webhook-event`).
   The instance was left with no webhooks, no temporary user and no temporary
   group.

## Sanitization

Applied uniformly to `User` objects (top level, `extendedUser`, and the parsed
`Record.object`):

- `email`, `contactEmail` → `<role>@example.com`; `phone` → `10000000000`
- `avatar`, `permanentAvatar` → `https://example.com/avatar.png`
- `password`, `passwordSalt`, `accessKey`, `accessSecret`, `accessToken`,
  `originalToken`, `originalRefreshToken`, `hash`, `preHash` → `***` (only when
  non-empty)
- `clientIp`, `createdIp`, `lastSigninIp` → `203.0.113.10`
- the real sample user's `name` → `sample-user`, `displayName` → `Sample User`
  (temporary objects were already named `bifrost-sample-*`)

Kept verbatim: `id`, `owner`, `groups`, `roles`, `permissions`, `isAdmin`,
`isForbidden`, `isDeleted`, `createdTime`, `updatedTime`, `externalId`,
`signupApplication`, `type`, group metadata and all `Record` fields other than
`clientIp`.
