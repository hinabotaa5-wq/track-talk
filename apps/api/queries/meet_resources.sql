-- name: ListMeetResources :many
SELECT meet_key, official_url, timetable_url, youtube_video_id
FROM meet_resources WHERE meet_key = ANY(sqlc.arg('meet_keys')::text[]);

-- name: UpsertMeetResources :exec
INSERT INTO meet_resources (meet_key, official_url, timetable_url, youtube_video_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (meet_key) DO UPDATE SET official_url = EXCLUDED.official_url,
 timetable_url = EXCLUDED.timetable_url, youtube_video_id = EXCLUDED.youtube_video_id, updated_at = now();

-- name: GetMeetResourcesForPost :one
SELECT mt.meet_key, p.body,
 COALESCE(mr.official_url, '')::text AS official_url,
 COALESCE(mr.timetable_url, '')::text AS timetable_url,
 COALESCE(mr.youtube_video_id, '')::text AS youtube_video_id
FROM meet_threads mt JOIN posts p ON p.id = mt.post_id
LEFT JOIN meet_resources mr ON mr.meet_key = mt.meet_key
WHERE p.id = $1 AND p.kind = 'meet' AND p.deleted_at IS NULL AND p.hidden_at IS NULL;
