DROP TABLE IF EXISTS live_processed_events;
DROP TABLE IF EXISTS live_session_stats;
DROP INDEX IF EXISTS uq_live_chat_messages_event_id;
ALTER TABLE live_chat_messages DROP COLUMN IF EXISTS event_id;
ALTER TABLE live_chat_messages DROP COLUMN IF EXISTS user_id;
