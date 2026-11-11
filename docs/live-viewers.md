# Live viewers (presence)

## Behavior

- A viewer is present while the live room is open, including paused playback.
- Browser sends `POST /v1/live/streams/:id/presence` immediately and every 15 seconds. Body: `tab_id` (UUID), `viewer_id` (guest browser UUID). Authenticated identity always comes from the verified Bearer token.
- `DELETE` at the same path requests immediate departure, with the same IDs and `broadcast` returned by the heartbeat. A failed/unavailable unload request is covered by expiry.
- A tab expires 45 seconds after its last heartbeat; SSE refreshes every 5 seconds, so a silent departure appears after approximately 45–50 seconds.
- Distinct accounts count separately. Tabs of one account count once. Anonymous tabs share a localStorage guest ID; different browsers/devices count separately. Without storage, each mounted room is a separate guest. Guest IDs are client-provided and are not fraud-proof viewer attribution.
- Keys include the livestream ID and `started_at`, isolating each broadcast. Reconnect inside the existing grace period retains the broadcast. Old leave requests cannot delete a new broadcast's presence.

## SSE and reads

`GET /v1/live/streams/:id/viewers` is a public, read-only SSE stream for available live rooms. It sends JSON `{ stream_id, broadcast, viewers_count }`, including zero. It does not itself register a viewer. `{ type: "ended", viewers_count: 0 }` closes an ended, banned or replaced broadcast. `{ type: "unavailable" }` signals a transient storage failure before closing; EventSource can reconnect.

The room, active stream list and live info responses use Redis counts when available. This does not write the historical `livestreams.viewers_count` column or change the admin dashboard's database-based aggregate. No cumulative VOD view is recorded. No change to live-agg, view-sync, SRS or DB schema.

Each SSE connection polls shared Redis every five seconds. This supports API replicas without relying on the in-process chat hub. It is intended for the current project scale; large audiences would benefit from a shared room-level broadcaster instead of per-connection polling. The atomic Lua operation removes expired tabs and deduplicates identities; idle Redis keys expire after 90 seconds.

## Release

Build/push backend `0.0.8` and frontend `0.0.11` using their `build.prod.sh` scripts, then update `BACKEND_VERSION` and `FRONTEND_VERSION` in the existing deployment environment. Redeploy app/frontend. No new environment variables or Compose changes; do not recreate SRS or stop OBS for this feature. API restart can briefly interrupt chat/SSE and clients reconnect.

Manual checks after deployment:

1. Open a live room: count becomes 1 after the heartbeat/SSE refresh.
2. Open another tab with the same account/browser: count stays 1.
3. Use another account/browser: count becomes 2.
4. Close one of the first viewer's tabs: count stays 2; close the last tab: count drops to 1 after leave or expiry.
5. Disconnect the second browser's network: count drops after 45–50 seconds without any other viewer joining.
6. End the live and start a new broadcast: old counts and old SSE do not carry across.

Automated tests cover Redis expiry, tab deduplication, identity changes, concurrent heartbeats, broadcast isolation, rejection of stale leave requests, bans/offline rooms and HTTP identity validation.

## SSE logger fix

Backend 0.0.8 prevents the request logger from reading `Response.Body()` for streaming responses. Reading it drains SSE and delays headers/events until the connection ends. The HTTP/SSE regression test now includes the production logger middleware. If Redis counts update but the room only changes after a reload, update the backend to this release.
