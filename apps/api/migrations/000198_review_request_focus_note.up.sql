-- スタンダードの申込で選手が任意で書ける「意識していること」（本体 #1093）。
-- #1013 補足の任意4項目（PB・目標タイム・風速・その時の体のコンディション）に5項目目として足す。未入力は NULL。
ALTER TABLE review_requests
    ADD COLUMN focus_note text;

ALTER TABLE review_requests
    ADD CONSTRAINT review_requests_focus_note_length CHECK (focus_note IS NULL OR char_length(focus_note) BETWEEN 1 AND 300);
