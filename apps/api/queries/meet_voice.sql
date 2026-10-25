-- name: LockMeetVoiceUser :exec
SELECT pg_advisory_xact_lock(hashtextextended('meet_voice_user:' || sqlc.arg(user_id)::uuid::text, 0));

-- name: LockMeetVoiceRoom :exec
SELECT pg_advisory_xact_lock(hashtextextended('meet_voice_room:' || sqlc.arg(post_id)::uuid::text, 0));

-- name: GetMeetVoiceReservation :one
SELECT * FROM meet_voice_reservations WHERE user_id = $1;

-- name: ListPendingMeetVoiceReservations :many
SELECT id FROM meet_voice_reservations
WHERE post_id = $1 AND state = 'pending' AND token_expires_at > clock_timestamp();

-- name: CreateMeetVoiceReservation :exec
INSERT INTO meet_voice_reservations (user_id, id, post_id, token_expires_at)
VALUES ($1, $2, $3, $4);

-- name: DeleteMeetVoiceReservation :exec
DELETE FROM meet_voice_reservations WHERE user_id = $1 AND id = $2;

-- name: IsMeetVoiceBanned :one
SELECT EXISTS (SELECT 1 FROM meet_voice_bans
WHERE post_id = $1 AND user_id = $2 AND (expires_at IS NULL OR expires_at > clock_timestamp()));

-- name: ConsumeMeetVoiceJoinAttempt :one
INSERT INTO meet_voice_rate_limits (user_id, window_start, attempts)
VALUES ($1, clock_timestamp(), 1)
ON CONFLICT (user_id) DO UPDATE SET
    window_start = CASE WHEN meet_voice_rate_limits.window_start <= clock_timestamp() - interval '1 minute' THEN clock_timestamp() ELSE meet_voice_rate_limits.window_start END,
    attempts = CASE WHEN meet_voice_rate_limits.window_start <= clock_timestamp() - interval '1 minute' THEN 1 ELSE LEAST(meet_voice_rate_limits.attempts + 1, 6) END
RETURNING attempts;

-- name: UpsertMeetVoiceSession :exec
INSERT INTO meet_voice_sessions (room_sid, post_id, room_name, started_at, last_event_at)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (room_sid) DO NOTHING;

-- name: RecordMeetVoiceWebhook :one
INSERT INTO meet_voice_webhook_events (id, event, room_sid, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO NOTHING
RETURNING id;

-- name: UpdateMeetVoiceParticipantJoined :exec
UPDATE meet_voice_reservations
SET state = 'connected', participant_sid = $3, last_event_at = $4
WHERE id = $1 AND post_id = $2
  AND (last_event_at IS NULL OR last_event_at < $4 OR (last_event_at = $4 AND state = 'pending'));

-- name: UpdateMeetVoiceParticipantLeft :exec
UPDATE meet_voice_reservations
SET state = 'disconnected', participant_sid = $3, last_event_at = $4
WHERE id = $1 AND post_id = $2 AND (participant_sid = $3 OR participant_sid = '')
  AND (last_event_at IS NULL OR last_event_at <= $4);

-- name: FinishMeetVoiceSession :exec
UPDATE meet_voice_sessions SET ended_at = $2, last_event_at = $2
WHERE room_sid = $1 AND last_event_at <= $2 AND ended_at IS NULL;

-- name: PruneMeetVoiceWebhookEvents :exec
DELETE FROM meet_voice_webhook_events WHERE received_at < clock_timestamp() - interval '30 days';

-- name: LockReadableMeetVoicePost :one
SELECT id FROM posts
WHERE id = $1 AND kind = 'meet' AND deleted_at IS NULL AND hidden_at IS NULL
FOR SHARE;

-- name: MeetVoicePostExists :one
SELECT EXISTS (SELECT 1 FROM posts WHERE id = $1 AND kind = 'meet');

-- name: GetMeetVoiceSettings :one
SELECT * FROM meet_voice_settings WHERE singleton=true;

-- name: LockMeetVoiceSettings :one
SELECT * FROM meet_voice_settings WHERE singleton=true FOR SHARE;

-- name: UpdateMeetVoiceSettings :one
UPDATE meet_voice_settings SET paused=$1, restrict_to_posts=$2, allowed_post_ids=$3, monthly_minutes_limit=$4
WHERE singleton=true RETURNING *;

-- name: MeetVoiceUsageMinutes :one
SELECT COALESCE(ceil(sum(GREATEST(0,extract(epoch FROM (COALESCE(ended_at,clock_timestamp()) - GREATEST(created_at,date_trunc('month',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')))))/60),0)::bigint AS minutes
FROM meet_voice_participants
WHERE COALESCE(ended_at,clock_timestamp()) >= date_trunc('month',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC';

-- name: InsertMeetVoiceParticipant :exec
INSERT INTO meet_voice_participants(id,user_id,post_id,room_sid,public_name) VALUES($1,$2,$3,$4,$5);

-- name: GetMeetVoiceParticipant :one
SELECT * FROM meet_voice_participants WHERE id=$1;

-- name: EndMeetVoiceParticipant :exec
UPDATE meet_voice_participants SET ended_at=COALESCE(ended_at,clock_timestamp()),revoked_at=clock_timestamp() WHERE id=$1;

-- name: TouchMeetVoiceReservation :one
UPDATE meet_voice_reservations SET last_seen_at=clock_timestamp(),
    confirmed_at=CASE WHEN sqlc.arg(confirm)::boolean THEN clock_timestamp() ELSE confirmed_at END,
    solo_since=CASE WHEN sqlc.arg(solo)::boolean THEN COALESCE(solo_since,clock_timestamp()) ELSE NULL END
WHERE user_id=$1 AND id=$2 RETURNING *;

-- name: SetMeetVoiceSoloState :one
UPDATE meet_voice_reservations SET solo_since=CASE WHEN sqlc.arg(solo)::boolean THEN COALESCE(solo_since,clock_timestamp()) ELSE NULL END
WHERE user_id=$1 AND id=$2 RETURNING *;

-- name: ListMeetVoiceReservations :many
SELECT r.*, p.public_name, s.title FROM meet_voice_reservations r
JOIN meet_voice_participants p ON p.id=r.id JOIN posts s ON s.id=r.post_id
ORDER BY r.created_at LIMIT 1000;

-- name: UpsertMeetVoiceBan :exec
INSERT INTO meet_voice_bans(post_id,user_id,reason) VALUES($1,$2,$3)
ON CONFLICT(post_id,user_id) DO UPDATE SET reason=EXCLUDED.reason, expires_at=NULL;

-- name: DeleteMeetVoiceBan :exec
DELETE FROM meet_voice_bans WHERE post_id=$1 AND user_id=$2;

-- name: ListMeetVoiceBans :many
SELECT b.*, p.nickname FROM meet_voice_bans b JOIN profiles p ON p.id=b.user_id
WHERE b.expires_at IS NULL OR b.expires_at>clock_timestamp() ORDER BY b.created_at DESC LIMIT 200;

-- name: CreateMeetVoiceAction :exec
INSERT INTO meet_voice_actions(id,actor_id,connection_id,action,success,target_user_id,post_id) VALUES($1,$2,$3,$4,$5,$6,$7);

-- name: VoiceReportAllowed :one
SELECT EXISTS(SELECT 1 FROM meet_voice_reservations r
JOIN meet_voice_participants reporter ON reporter.id=r.id
JOIN meet_voice_participants target ON target.id=sqlc.arg(target_id)::uuid AND target.room_sid=reporter.room_sid
WHERE r.user_id=sqlc.arg(user_id)::uuid AND target.post_id=sqlc.arg(post_id)::uuid
  AND target.user_id<>r.user_id AND target.id<>r.id);

-- name: CleanupMeetVoiceHistory :exec
DELETE FROM meet_voice_participants p WHERE ended_at<clock_timestamp()-interval '90 days'
AND NOT EXISTS(SELECT 1 FROM reports r WHERE r.target_type='voice_participant' AND r.target_id=p.id AND r.status='open');

-- name: CountMeetVoiceReservations :one
SELECT count(*) FROM meet_voice_reservations;

-- name: MarkMeetVoiceTick :exec
UPDATE meet_voice_settings SET last_tick_at=clock_timestamp() WHERE singleton=true;

-- name: EndMeetVoiceRoomParticipants :exec
UPDATE meet_voice_participants SET ended_at=COALESCE(ended_at,clock_timestamp()) WHERE room_sid=$1;

-- name: EndMeetVoiceWebhookParticipant :exec
UPDATE meet_voice_participants p SET ended_at=COALESCE(p.ended_at,clock_timestamp())
WHERE p.id=$1 AND EXISTS(SELECT 1 FROM meet_voice_reservations r WHERE r.id=p.id AND r.state='disconnected' AND r.last_event_at=$2);

-- name: RestoreMeetVoiceParticipant :exec
UPDATE meet_voice_participants SET ended_at=NULL WHERE id=$1;


-- name: ListOrphanMeetVoiceParticipants :many
SELECT id,post_id FROM meet_voice_participants p WHERE revoked_at IS NULL
AND NOT EXISTS(SELECT 1 FROM meet_voice_reservations r WHERE r.id=p.id) LIMIT 1000;

-- name: CleanupMeetVoiceActions :exec
DELETE FROM meet_voice_actions WHERE created_at<clock_timestamp()-interval '90 days';
