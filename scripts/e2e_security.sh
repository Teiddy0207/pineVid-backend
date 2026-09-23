#!/usr/bin/env bash
# PipeVid — security regression E2E test (Phase 1).
#
# Runs against a live backend and asserts every Phase-1 fix with real HTTP
# calls: token rotation, SSE auth, webhook secrets, private/banned video
# access, identity spoofing, bans, IP spoofing, CORS and rate limiting.
#
# Requirements: curl, jq, the backend running, and the `db` compose container
# (used once to promote a throwaway account to admin).
#
# Usage:  backend/scripts/e2e_security.sh            (API defaults to :8080)
#         API=http://localhost:8080 backend/scripts/e2e_security.sh
set -uo pipefail

API="${API:-http://localhost:8080}"
V1="$API/v1"
DB_CONTAINER="${DB_CONTAINER:-db}"
ENV_FILE="${ENV_FILE:-$(dirname "$0")/../.env}"
SECRET="${INTERNAL_WEBHOOK_SECRET:-$(grep -E '^INTERNAL_WEBHOOK_SECRET=' "$ENV_FILE" | cut -d= -f2-)}"

PASS=0
FAIL=0
RUN_ID="$(date +%s)$RANDOM"

green() { printf '\033[32m%s\033[0m\n' "$*"; }
red() { printf '\033[31m%s\033[0m\n' "$*"; }

check() { # check <description> <expected> <actual>
	if [[ "$2" == "$3" ]]; then
		PASS=$((PASS + 1)); green "  PASS  $1"
	else
		FAIL=$((FAIL + 1)); red "  FAIL  $1 (expected '$2', got '$3')"
	fi
}

# status <curl args...> — prints only the HTTP status code
status() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

json_post() { # json_post <url> <json> [token]
	local auth=()
	[[ -n "${3:-}" ]] && auth=(-H "Authorization: Bearer $3")
	curl -s -X POST "$1" -H 'Content-Type: application/json' "${auth[@]}" -d "$2"
}

register_and_login() { # register_and_login <name> -> "token refresh userID"
	local email="$1_$RUN_ID@e2e.test"
	local reg
	reg=$(json_post "$V1/auth/register" "{\"username\":\"$1_$RUN_ID\",\"email\":\"$email\",\"password\":\"password123\"}")
	local login
	login=$(json_post "$V1/auth/login" "{\"email\":\"$email\",\"password\":\"password123\"}")
	echo "$(jq -r .token <<<"$login") $(jq -r .refresh_token <<<"$login") $(jq -r .id <<<"$reg")"
}

echo "== PipeVid security E2E against $API =="
[[ -z "$SECRET" ]] && { red "INTERNAL_WEBHOOK_SECRET not found (env or $ENV_FILE)"; exit 2; }

read -r TOKEN_A REFRESH_A USER_A <<<"$(register_and_login alice)"
read -r TOKEN_B REFRESH_B USER_B <<<"$(register_and_login bob)"
read -r TOKEN_ADMIN _ USER_ADMIN <<<"$(register_and_login admin)"
docker exec "$DB_CONTAINER" psql -U user -d db -qc "UPDATE users SET role='admin' WHERE id='$USER_ADMIN'" >/dev/null

echo "-- 1.6 Refresh token rotation"
check "login returns a refresh token" "true" "$([[ ${#REFRESH_A} -gt 20 ]] && echo true || echo false)"
check "access token is rejected as a refresh token" "401" "$(status -X POST "$V1/auth/refresh" -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$TOKEN_A\"}")"
ROTATED=$(json_post "$V1/auth/refresh" "{\"refresh_token\":\"$REFRESH_A\"}")
REFRESH_A2=$(jq -r .refresh_token <<<"$ROTATED")
check "refresh issues a new pair" "true" "$([[ -n "$REFRESH_A2" && "$REFRESH_A2" != "null" && "$REFRESH_A2" != "$REFRESH_A" ]] && echo true || echo false)"
check "reusing a rotated refresh token is rejected" "401" "$(status -X POST "$V1/auth/refresh" -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH_A\"}")"
check "...and that reuse revoked the new token too (theft detection)" "401" "$(status -X POST "$V1/auth/refresh" -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH_A2\"}")"
check "logout returns 204" "204" "$(status -X POST "$V1/auth/logout" -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH_B\"}")"
check "logged-out refresh token is rejected" "401" "$(status -X POST "$V1/auth/refresh" -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH_B\"}")"

echo "-- 1.1 SSE authentication"
check "notifications SSE: ?user_id= without token is rejected" "401" "$(status --max-time 3 "$V1/events/notifications?user_id=$USER_A")"
check "notifications SSE: garbage token is rejected" "401" "$(status --max-time 3 "$V1/events/notifications?token=garbage")"
check "notifications SSE: valid token connects" "200" "$(status --max-time 2 "$V1/events/notifications?token=$TOKEN_B")"
check "video-status SSE without token is rejected" "401" "$(status --max-time 3 "$V1/events/videos")"
check "regular API does not accept ?token=" "401" "$(status "$V1/user/profile?token=$TOKEN_A")"

echo "-- 1.4 Internal webhook secret"
check "transcode callback without secret" "401" "$(status -X POST "$V1/transcode/callback" -H 'Content-Type: application/json' -d '{"video_id":"x","status":"complete"}')"
check "transcode callback with wrong secret" "401" "$(status -X POST "$V1/transcode/callback" -H 'X-Internal-Token: nope' -H 'Content-Type: application/json' -d '{"video_id":"x","status":"complete"}')"
check "worker heartbeat without secret" "401" "$(status -X POST "$V1/admin/workers/heartbeat" -H 'Content-Type: application/json' -d '{}')"
check "SRS on_publish without secret" "401" "$(status -X POST "$V1/live/auth" -H 'Content-Type: application/json' -d '{"stream":"x"}')"
check "SRS on_dvr without secret" "401" "$(status -X POST "$V1/live/dvr" -H 'Content-Type: application/json' -d '{"stream":"x"}')"
check "SRS on_publish with ?secret= reaches the handler (unknown key => 403)" "403" "$(status -X POST "$V1/live/auth?secret=$SECRET" -H 'Content-Type: application/json' -d '{"stream":"unknown-key"}')"

echo "-- 1.2 Private / unfinished / banned videos"
UPLOAD=$(json_post "$V1/studio/upload-url" '{"title":"E2E private video","file_name":"clip.mp4","visibility":"public"}' "$TOKEN_A")
VID=$(jq -r .video_id <<<"$UPLOAD")
check "pending (not yet transcoded) video is hidden from others" "404" "$(status "$V1/videos/$VID" -H "Authorization: Bearer $TOKEN_B")"
check "...but visible to its owner" "200" "$(status "$V1/videos/$VID" -H "Authorization: Bearer $TOKEN_A")"
check "confirm-upload of someone else's video is forbidden" "403" "$(status -X POST "$V1/studio/confirm-upload" -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -d "{\"video_id\":\"$VID\"}")"
curl -s -o /dev/null -X POST "$V1/transcode/callback" -H "X-Internal-Token: $SECRET" -H 'Content-Type: application/json' -d "{\"video_id\":\"$VID\",\"status\":\"complete\",\"hls_master_url\":\"/hls-streams/videos/$VID/master.m3u8\"}"
check "completed public video is visible to anyone" "200" "$(status "$V1/videos/$VID")"
check "invalid visibility value is rejected" "400" "$(status -X PUT "$V1/studio/videos/$VID" -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' -d '{"visibility":"hacked"}')"
curl -s -o /dev/null -X PUT "$V1/studio/videos/$VID" -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' -d '{"visibility":"private"}'
check "private video: stranger gets 404" "404" "$(status "$V1/videos/$VID" -H "Authorization: Bearer $TOKEN_B")"
check "private video: anonymous gets 404" "404" "$(status "$V1/videos/$VID")"
check "private video: owner gets 200" "200" "$(status "$V1/videos/$VID" -H "Authorization: Bearer $TOKEN_A")"
check "private video: subtitles hidden" "404" "$(status "$V1/videos/$VID/subtitles")"
check "private video: comments hidden" "404" "$(status "$V1/videos/$VID/comments")"
check "private video: cannot be liked by a stranger" "404" "$(status -X POST "$V1/videos/$VID/like" -H "Authorization: Bearer $TOKEN_B")"
check "private video: views not counted" "404" "$(status -X POST "$V1/videos/$VID/views")"
curl -s -o /dev/null -X PUT "$V1/studio/videos/$VID" -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' -d '{"visibility":"unlisted"}'
check "unlisted video: watchable with the link" "200" "$(status "$V1/videos/$VID")"
IN_LIST=$(curl -s "$V1/videos?q=E2E%20private%20video&limit=50" | jq --arg id "$VID" '[.data[]? | select(.id==$id)] | length')
check "unlisted video: not in public listing" "0" "$IN_LIST"
curl -s -o /dev/null -X PUT "$V1/studio/videos/$VID" -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' -d '{"visibility":"public"}'

echo "-- 1.3 Identity spoofing"
COMMENT=$(curl -s -X POST "$V1/videos/$VID/comments" -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -H 'X-User-Name: admin' -H 'X-User-Avatar: http://evil/x.png' -d '{"content":"hello"}')
check "comment author comes from the account, not X-User-Name" "bob_$RUN_ID" "$(jq -r .user.name <<<"$COMMENT")"
check "live chat requires login" "401" "$(status -X POST "$V1/live/streams/any/chat" -H 'Content-Type: application/json' -d '{"text":"hi","username":"admin"}')"
CHAT=$(curl -s -X POST "$V1/live/streams/any/chat" -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -d '{"text":"hi","username":"admin"}')
check "chat sender comes from the account, not the body" "bob_$RUN_ID" "$(jq -r .user.name <<<"$CHAT")"

echo "-- 1.7 Channel page"
check "public channel page does not expose email" "false" "$(curl -s "$V1/channels/$USER_A" | jq 'has("email")')"

echo "-- 2.2 View dedup cannot be bypassed with X-Forwarded-For"
R1=$(curl -s -X POST "$V1/videos/$VID/views" -H 'X-Forwarded-For: 1.1.1.1' -H 'X-Device-ID: dev-1' | jq -r .recorded)
R2=$(curl -s -X POST "$V1/videos/$VID/views" -H 'X-Forwarded-For: 2.2.2.2' -H 'X-Device-ID: dev-1' | jq -r .recorded)
check "first view recorded" "true" "$R1"
check "second view with a spoofed IP is deduplicated" "false" "$R2"

echo "-- 1.5 Bans take effect immediately"
check "non-admin cannot ban" "403" "$(status -X POST "$V1/admin/users/$USER_A/ban" -H "Authorization: Bearer $TOKEN_B")"
check "admin bans bob" "200" "$(status -X POST "$V1/admin/users/$USER_B/ban" -H "Authorization: Bearer $TOKEN_ADMIN")"
check "bob's existing access token is rejected right away" "403" "$(status "$V1/user/profile" -H "Authorization: Bearer $TOKEN_B")"
check "bob cannot log in" "403" "$(status -X POST "$V1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"bob_$RUN_ID@e2e.test\",\"password\":\"password123\"}")"
check "admin unbans bob" "200" "$(status -X POST "$V1/admin/users/$USER_B/unban" -H "Authorization: Bearer $TOKEN_ADMIN")"
check "bob's token works again after unban" "200" "$(status "$V1/user/profile" -H "Authorization: Bearer $TOKEN_B")"

check "admin takes the video down" "200" "$(status -X POST "$V1/admin/videos/$VID/ban" -H "Authorization: Bearer $TOKEN_ADMIN")"
check "banned video: 404 for everyone else" "404" "$(status "$V1/videos/$VID")"
check "banned video: owner still sees it (Studio)" "200" "$(status "$V1/videos/$VID" -H "Authorization: Bearer $TOKEN_A")"
curl -s -o /dev/null -X POST "$V1/transcode/callback" -H "X-Internal-Token: $SECRET" -H 'Content-Type: application/json' -d "{\"video_id\":\"$VID\",\"status\":\"complete\",\"hls_master_url\":\"/evil/master.m3u8\"}"
check "a later transcode callback cannot revive a banned video" "404" "$(status "$V1/videos/$VID")"

curl -s -o /dev/null "$V1/studio/live/key" -H "Authorization: Bearer $TOKEN_A" # creates the livestream row
STREAM_ID=$(docker exec "$DB_CONTAINER" psql -U user -d db -tAc "SELECT id FROM livestreams WHERE user_id='$USER_A'")
STREAM_KEY=$(docker exec "$DB_CONTAINER" psql -U user -d db -tAc "SELECT stream_key FROM livestreams WHERE user_id='$USER_A'")
check "stream key is accepted before a ban" "200" "$(status -X POST "$V1/live/auth?secret=$SECRET" -H 'Content-Type: application/json' -d "{\"stream\":\"$STREAM_KEY\"}")"
check "admin bans the stream" "200" "$(status -X POST "$V1/admin/streams/$STREAM_ID/ban" -H "Authorization: Bearer $TOKEN_ADMIN")"
check "banned stream key is rejected by on_publish" "403" "$(status -X POST "$V1/live/auth?secret=$SECRET" -H 'Content-Type: application/json' -d "{\"stream\":\"$STREAM_KEY\"}")"
check "admin unbans the stream" "200" "$(status -X POST "$V1/admin/streams/$STREAM_ID/unban" -H "Authorization: Bearer $TOKEN_ADMIN")"
check "stream key accepted again after unban" "200" "$(status -X POST "$V1/live/auth?secret=$SECRET" -H 'Content-Type: application/json' -d "{\"stream\":\"$STREAM_KEY\"}")"
curl -s -o /dev/null -X POST "$V1/live/unpublish?secret=$SECRET" -H 'Content-Type: application/json' -d "{\"stream\":\"$STREAM_KEY\"}"

echo "-- 1.9 CORS"
ACAO_GOOD=$(curl -s -o /dev/null -D - "$V1/videos" -H 'Origin: http://localhost:3000' | grep -i '^access-control-allow-origin' | tr -d '\r' | cut -d' ' -f2)
ACAO_EVIL=$(curl -s -o /dev/null -D - "$V1/videos" -H 'Origin: http://evil.example' | grep -ci '^access-control-allow-origin')
check "frontend origin is allowed" "http://localhost:3000" "$ACAO_GOOD"
check "other origins get no CORS grant" "0" "$ACAO_EVIL"

echo "-- 1.8 Rate limiting (runs last: blocks logins from this IP for 1 minute)"
LAST=""
for _ in $(seq 1 12); do
	LAST=$(status -X POST "$V1/auth/login" -H 'Content-Type: application/json' -d '{"email":"nobody@e2e.test","password":"wrongpass"}')
done
check "11th+ login attempt within a minute is throttled" "429" "$LAST"

echo
echo "== Result: $PASS passed, $FAIL failed =="
[[ $FAIL -eq 0 ]]
