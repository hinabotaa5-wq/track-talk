-- 注意：この down は、000197 で追加した「意識していること」（focus_note）の列を消す。値がある行があると、その値は失われ、戻せない。
ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_focus_note_length;

ALTER TABLE review_requests
    DROP COLUMN focus_note;
