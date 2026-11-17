-- 継続プラン（period）の契約で、選手の感覚メモを担当コーチと共有するための読み書き。
-- 共有の有無は sense_note_contract_exclusions の行の有無で表す（行が無ければ共有中）。

-- name: ListContractSenseNotes :many
-- 共有ウィンドウ（window_start〜today、どちらも日本時間の日）内の、選手の感覚メモを新しい順に全部返す。
-- excluded が true のものは、選手がチェックを外した（コーチには見せない）メモ。
SELECT sn.id, sn.body, sn.event, sn.day, sn.created_at, sn.updated_at,
    (ex.sense_note_id IS NOT NULL)::boolean AS excluded
FROM sense_notes sn
LEFT JOIN sense_note_contract_exclusions ex
    ON ex.contract_id = sqlc.arg('contract_id') AND ex.sense_note_id = sn.id
WHERE sn.author_id = sqlc.arg('author_id')
    AND sn.day >= sqlc.arg('window_start')::date
    AND sn.day <= sqlc.arg('today')::date
ORDER BY sn.day DESC, sn.created_at DESC, sn.id DESC;

-- name: GetContractSenseNote :one
-- 共有の切り替え対象のメモを1件読む。本人確認とウィンドウ判定は呼び出し側で行う。
SELECT sn.id, sn.day
FROM sense_notes sn
WHERE sn.id = $1 AND sn.author_id = $2;

-- name: InsertSenseNoteContractExclusion :exec
-- チェックを外す。すでに外れていても何も変わらない（冪等）。
INSERT INTO sense_note_contract_exclusions (contract_id, sense_note_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteSenseNoteContractExclusion :exec
-- チェックを付け直す。すでに付いていても何も変わらない（冪等）。
DELETE FROM sense_note_contract_exclusions
WHERE contract_id = $1 AND sense_note_id = $2;

-- 同じ日の書き直しで除外を引き継ぐための記録（sense_note_rewrite_exclusions）。
-- 記録は「選手・契約・日付」だけで、メモの本文は残さない。

-- name: ClearSenseNoteRewriteExclusions :exec
-- 消すメモの日の記録と、今日より前の日の記録を捨てる。消す時点の除外の状態だけを正にする。
DELETE FROM sense_note_rewrite_exclusions
WHERE author_id = sqlc.arg('author_id')
    AND (day = sqlc.arg('note_day')::date OR day < sqlc.arg('today')::date);

-- name: SaveSenseNoteRewriteExclusions :exec
-- 消す前のメモに除外があった契約を、そのメモの日付で残す。今日の分のメモのときだけ残す（書き直せるのは今日の分だけ）。
INSERT INTO sense_note_rewrite_exclusions (author_id, contract_id, day)
SELECT sn.author_id, ex.contract_id, sn.day
FROM sense_note_contract_exclusions ex
JOIN sense_notes sn ON sn.id = ex.sense_note_id
WHERE sn.id = sqlc.arg('note_id')
    AND sn.author_id = sqlc.arg('author_id')
    AND sn.day = sqlc.arg('today')::date
ON CONFLICT DO NOTHING;

-- name: InheritSenseNoteRewriteExclusions :exec
-- 作ったメモに、同じ日に消したメモの除外を引き継ぐ。
INSERT INTO sense_note_contract_exclusions (contract_id, sense_note_id)
SELECT rw.contract_id, sqlc.arg('note_id')::uuid
FROM sense_note_rewrite_exclusions rw
WHERE rw.author_id = sqlc.arg('author_id')
    AND rw.day = sqlc.arg('day')::date
ON CONFLICT DO NOTHING;

-- name: DeleteSenseNoteRewriteExclusions :exec
-- 作ったあとは、その選手の記録を全部捨てる（使い終わった記録と、別の日の残りの両方）。
DELETE FROM sense_note_rewrite_exclusions
WHERE author_id = $1;
