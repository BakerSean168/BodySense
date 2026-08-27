-- 000057: Reframe the body profile around durable health context.
--
-- birth_date becomes the canonical age source for new clients. We intentionally
-- do not fabricate a birth date from legacy integer age because that would add
-- false precision. The legacy columns stay in place for compatibility.
ALTER TABLE user_profiles
    ADD COLUMN birth_date DATE,
    ADD COLUMN activity_pattern TEXT,
    ADD COLUMN sleep_pattern TEXT;

-- Preserve legacy sleep context only when it can be moved without inventing
-- facts. Occupation is deliberately NOT copied into activity_pattern: a job
-- title does not reliably describe sitting, standing, walking, or physical load.
UPDATE user_profiles
SET sleep_pattern = CASE
    WHEN sleep_time IS NOT NULL AND wake_time IS NOT NULL
        THEN '此前记录的常见作息：' || to_char(sleep_time, 'HH24:MI') || ' 入睡，' || to_char(wake_time, 'HH24:MI') || ' 起床'
    WHEN sleep_time IS NOT NULL
        THEN '此前记录的常见入睡时间：' || to_char(sleep_time, 'HH24:MI')
    WHEN wake_time IS NOT NULL
        THEN '此前记录的常见起床时间：' || to_char(wake_time, 'HH24:MI')
    ELSE NULL
END
WHERE sleep_pattern IS NULL
  AND (sleep_time IS NOT NULL OR wake_time IS NOT NULL);
