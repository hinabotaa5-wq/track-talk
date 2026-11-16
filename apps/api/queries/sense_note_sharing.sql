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
