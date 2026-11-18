ALTER TABLE notifications ADD COLUMN coaching_role text
    CHECK (coaching_role IN ('player', 'coach'));

-- Announcements always stay in the bell, even when their destination is a coaching screen.
UPDATE notifications n
SET coaching_role = CASE
    WHEN n.kind IN ('review_offer', 'review_offer_reminder', 'review_resubmitted',
        'review_resubmitted_after_accept', 'review_nomination_ended', 'review_clarify') THEN 'coach'
    WHEN left(n.kind, 7) = 'review_' THEN 'player'
    WHEN n.kind = 'message' THEN CASE
        WHEN split_part(n.url, '#', 1) = '/coach' OR split_part(n.url, '#', 1) LIKE '/coach/%' THEN 'coach'
        WHEN split_part(n.url, '#', 1) LIKE '/coaching/support-consults/%' OR split_part(n.url, '#', 1) LIKE '/coaching/requests/%'
            OR split_part(n.url, '#', 1) = '/me/coaching' OR split_part(n.url, '#', 1) LIKE '/me/coaching/%' THEN 'player'
        WHEN split_part(n.url, '#', 1) = '/me' AND n.title LIKE '紹介特典：%' THEN 'player'
        ELSE (SELECT CASE WHEN cc.coach_id = n.recipient_id THEN 'coach'
                          WHEN cc.requester_id = n.recipient_id THEN 'player' END
              FROM coaching_contracts cc
              WHERE split_part(n.url, '#', 1) = '/coaching/personal/' || cc.id::text)
    END
END
WHERE n.announcement_id IS NULL;
