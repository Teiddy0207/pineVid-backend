-- PipeVid demo/test data seed
-- ============================================================================
-- Populates a fresh (or already-running) PipeVid Postgres database with:
--   - 30 "creator" users (5 per category: Gaming, Science, Music, Travel,
--     Film, Lifestyle), each with 3 videos (varied Unsplash thumbnails)
--   - 70 "viewer" users, each with a preferred category, liking/watching a
--     handful of that category's videos (creates real collaborative-filtering
--     signal for the Matrix Factorization recommendation engine)
--   - every seeded user gets a real avatar (pravatar.cc), never left blank
--   - optionally: if a real account's email is passed as :target_email and it
--     exists, that account gets 2 "favorite channels" (1 Gaming + 1 Music
--     creator) with heavy likes/watch history/comment across ALL of their
--     videos — useful to demo a personalized "for you" feed end to end.
--
-- Usage:
--   psql "$PG_URL?sslmode=disable" -v target_email='someone@example.com' \
--        -f scripts/seed_demo_data.sql
--   (or just `make seed-demo` — see Makefile; TARGET_EMAIL is optional there too)
--
-- Safe to run on a fresh DB. Not idempotent: running it twice creates a
-- second batch of creators/viewers (usernames are suffixed with a random
-- short id so re-runs don't collide on the unique username/email constraints).
-- ============================================================================

\set ON_ERROR_STOP on
\if :{?target_email}
\else
  \set target_email '__none__'
\endif

BEGIN;

-- Unique-per-run suffix so this script can be safely re-executed against the
-- same database without hitting the users.username/email unique constraints.
CREATE TEMP TABLE seed_run AS SELECT substr(md5(random()::text), 1, 6) AS suffix;

CREATE TEMP TABLE seed_categories (category text);
INSERT INTO seed_categories (category) VALUES
  ('Gaming'), ('Science'), ('Music'), ('Travel'), ('Film'), ('Lifestyle');

CREATE TEMP TABLE seed_titles (category text, seq int, title text);
INSERT INTO seed_titles (category, seq, title) VALUES
  ('Gaming', 1, 'Pha carry 1vs5 nghẹt thở'),
  ('Gaming', 2, 'Giải đấu Quốc gia - Vòng bảng'),
  ('Gaming', 3, 'Top 10 khoảnh khắc Esports'),
  ('Science', 1, 'Khám phá hố đen vũ trụ'),
  ('Science', 2, 'Thí nghiệm vật lý lượng tử'),
  ('Science', 3, 'Bí ẩn AI và tương lai nhân loại'),
  ('Music', 1, 'Cover acoustic đêm mưa'),
  ('Music', 2, 'MV mới nhất tuần này'),
  ('Music', 3, 'Live session phòng thu'),
  ('Travel', 1, 'Phượt Tây Bắc mùa lúa chín'),
  ('Travel', 2, 'Khám phá hang động bí ẩn'),
  ('Travel', 3, 'Du lịch bụi xuyên Á'),
  ('Film', 1, 'Hậu trường phim bom tấn'),
  ('Film', 2, 'Trailer chính thức phần 2'),
  ('Film', 3, 'Phân tích điện ảnh chuyên sâu'),
  ('Lifestyle', 1, 'Vlog một ngày làm việc'),
  ('Lifestyle', 2, 'Thói quen buổi sáng hiệu quả'),
  ('Lifestyle', 3, 'Trang trí nhà cửa tối giản');

CREATE TEMP TABLE seed_thumbs (category text, bucket int, url text);
INSERT INTO seed_thumbs (category, bucket, url) VALUES
  ('Gaming', 0, 'https://images.unsplash.com/photo-1542751371-adc38448a05e?auto=format&fit=crop&w=800&q=80'),
  ('Gaming', 1, 'https://images.unsplash.com/photo-1552820728-8b83bb6b773f?auto=format&fit=crop&w=800&q=80'),
  ('Gaming', 2, 'https://images.unsplash.com/photo-1580327344181-c1163234e5a0?auto=format&fit=crop&w=800&q=80'),
  ('Science', 0, 'https://images.unsplash.com/photo-1446776811953-b23d57bd21aa?auto=format&fit=crop&w=800&q=80'),
  ('Science', 1, 'https://images.unsplash.com/photo-1462331940025-496dfbfc7564?auto=format&fit=crop&w=800&q=80'),
  ('Science', 2, 'https://images.unsplash.com/photo-1502134249126-9f3755a50d78?auto=format&fit=crop&w=800&q=80'),
  ('Music', 0, 'https://images.unsplash.com/photo-1598550476439-6847785fcea6?auto=format&fit=crop&w=800&q=80'),
  ('Music', 1, 'https://images.unsplash.com/photo-1501386761578-eac5c94b800a?auto=format&fit=crop&w=800&q=80'),
  ('Music', 2, 'https://images.unsplash.com/photo-1511671782779-c97d3d27a1d4?auto=format&fit=crop&w=800&q=80'),
  ('Travel', 0, 'https://images.unsplash.com/photo-1519501025264-65ba15a82390?auto=format&fit=crop&w=800&q=80'),
  ('Travel', 1, 'https://images.unsplash.com/photo-1507525428034-b723cf961d3e?auto=format&fit=crop&w=800&q=80'),
  ('Travel', 2, 'https://images.unsplash.com/photo-1476514525535-07fb3b4ae5f1?auto=format&fit=crop&w=800&q=80'),
  ('Film', 0, 'https://images.unsplash.com/photo-1516035069371-29a1b244cc32?auto=format&fit=crop&w=800&q=80'),
  ('Film', 1, 'https://images.unsplash.com/photo-1489599162946-2842f8a5a56b?auto=format&fit=crop&w=800&q=80'),
  ('Film', 2, 'https://images.unsplash.com/photo-1478720568477-152d9b164e26?auto=format&fit=crop&w=800&q=80'),
  ('Lifestyle', 0, 'https://images.unsplash.com/photo-1497366754035-f200968a6e72?auto=format&fit=crop&w=800&q=80'),
  ('Lifestyle', 1, 'https://images.unsplash.com/photo-1484154218962-a197022b5858?auto=format&fit=crop&w=800&q=80'),
  ('Lifestyle', 2, 'https://images.unsplash.com/photo-1483721310020-03333e577078?auto=format&fit=crop&w=800&q=80');

-- 1) 30 creator users (5 per category), each with a real pravatar avatar.
CREATE TEMP TABLE seed_creators (id uuid, category text, seq int);
INSERT INTO seed_creators (id, category, seq)
SELECT gen_random_uuid(), c.category, n
FROM seed_categories c, generate_series(1, 5) AS n;

INSERT INTO users (id, username, email, password_hash, avatar_url, created_at, updated_at)
SELECT
  sc.id,
  'creator_' || lower(sc.category) || '_' || sc.seq || '_' || sr.suffix,
  'creator_' || lower(sc.category) || '_' || sc.seq || '_' || sr.suffix || '@seed.pipevid.dev',
  '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
  'https://i.pravatar.cc/150?img=' || (1 + (row_number() OVER (ORDER BY sc.id))::int % 70),
  now(), now()
FROM seed_creators sc, seed_run sr;

-- 2) 90 videos (3 per creator), varied category-appropriate thumbnails.
CREATE TEMP TABLE seed_videos (id uuid, category text, creator_id uuid, seq int);
INSERT INTO seed_videos (id, category, creator_id, seq)
SELECT gen_random_uuid(), sc.category, sc.id, t.seq
FROM seed_creators sc
JOIN seed_titles t ON t.category = sc.category;

INSERT INTO videos (id, user_id, title, description, category, status, visibility, raw_s3_key, hls_url, thumbnail_url, duration, views, created_at, updated_at)
SELECT
  sv.id,
  sv.creator_id,
  t.title || ' #' || sc.seq,
  'Seed data cho demo recommendation',
  sv.category,
  'complete',
  'public',
  'raw-uploads/seed/' || sv.id || '/raw.mp4',
  '/hls-streams/videos/bf0609f3-a5e7-4753-827d-bc5fa6756b94/master.m3u8',
  th.url,
  lpad((10 + (random() * 40)::int)::text, 2, '0') || ':' || lpad((random() * 59)::int::text, 2, '0'),
  (20 + random() * 900)::int,
  now(), now()
FROM seed_videos sv
JOIN seed_creators sc ON sc.id = sv.creator_id
JOIN seed_titles t ON t.category = sv.category AND t.seq = sv.seq
JOIN seed_thumbs th ON th.category = sv.category AND th.bucket = (abs(hashtext(sv.id::text)) % 3);

-- 3) 70 viewer users, each with a preferred category (cycled evenly), each
--    with their own pravatar avatar.
CREATE TEMP TABLE seed_viewers (id uuid, pref_category text, rn int);
INSERT INTO seed_viewers (id, pref_category, rn)
SELECT
  gen_random_uuid(),
  (SELECT category FROM seed_categories ORDER BY category OFFSET (n - 1) % 6 LIMIT 1),
  n
FROM generate_series(1, 70) AS n;

INSERT INTO users (id, username, email, password_hash, avatar_url, created_at, updated_at)
SELECT
  sv.id,
  'viewer_seed_' || sv.rn || '_' || sr.suffix,
  'viewer_seed_' || sv.rn || '_' || sr.suffix || '@seed.pipevid.dev',
  '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
  'https://i.pravatar.cc/150?img=' || (1 + (sv.rn + 30) % 70),
  now(), now()
FROM seed_viewers sv, seed_run sr;

-- 4) Each viewer likes ~4 and watches ~3 videos from their preferred category
--    (many different viewers sharing tastes is what gives the Matrix
--    Factorization model real collaborative-filtering signal to learn from).
INSERT INTO likes (id, video_id, user_id, created_at)
SELECT gen_random_uuid(), x.video_id, x.viewer_id, now()
FROM (
  SELECT sve.id AS viewer_id, svd.id AS video_id,
         row_number() OVER (PARTITION BY sve.id ORDER BY random()) AS rn
  FROM seed_viewers sve
  JOIN seed_videos svd ON svd.category = sve.pref_category
) x
WHERE x.rn <= 4
ON CONFLICT DO NOTHING;

INSERT INTO watch_history (user_id, video_id, watch_seconds, last_watched_at)
SELECT x.viewer_id, x.video_id, (30 + random() * 400)::int, now()
FROM (
  SELECT sve.id AS viewer_id, svd.id AS video_id,
         row_number() OVER (PARTITION BY sve.id ORDER BY random()) AS rn
  FROM seed_viewers sve
  JOIN seed_videos svd ON svd.category = sve.pref_category
) x
WHERE x.rn > 4 AND x.rn <= 7
ON CONFLICT (user_id, video_id) DO NOTHING;

-- 5) Optional: give a real, existing account (by email) 2 "favorite
--    channels" — 1 Gaming + 1 Music creator — with heavy interaction across
--    ALL of their videos, so its personalized feed has an obvious, checkable
--    signal. Skipped entirely if :target_email wasn't passed or doesn't exist.
--
-- psql variable substitution (:'var') does not happen inside DO $$ ... $$
-- bodies, so the value is handed off through a session-local GUC instead.
SELECT set_config('pipevid.seed_target_email', :'target_email', false);

DO $$
DECLARE
  target_id uuid;
  fav_gaming uuid;
  fav_music uuid;
BEGIN
  SELECT id INTO target_id FROM users WHERE email = current_setting('pipevid.seed_target_email', true);
  IF target_id IS NULL THEN
    RAISE NOTICE 'seed_demo_data: target_email not found, skipping favorite-channel seeding';
    RETURN;
  END IF;

  SELECT id INTO fav_gaming FROM seed_creators WHERE category = 'Gaming' AND seq = 1;
  SELECT id INTO fav_music FROM seed_creators WHERE category = 'Music' AND seq = 1;

  INSERT INTO likes (id, video_id, user_id, created_at)
  SELECT gen_random_uuid(), v.id, target_id, now()
  FROM seed_videos v
  WHERE v.creator_id IN (fav_gaming, fav_music)
  ON CONFLICT DO NOTHING;

  INSERT INTO watch_history (user_id, video_id, watch_seconds, last_watched_at)
  SELECT target_id, v.id, (300 + random() * 200)::int, now()
  FROM seed_videos v
  WHERE v.creator_id IN (fav_gaming, fav_music)
  ON CONFLICT (user_id, video_id) DO UPDATE
    SET watch_seconds = GREATEST(watch_history.watch_seconds, EXCLUDED.watch_seconds),
        last_watched_at = EXCLUDED.last_watched_at;

  INSERT INTO comments (id, video_id, user_id, user_name, user_avatar, content, created_at)
  SELECT gen_random_uuid(), v.id, target_id, u.username, u.avatar_url, 'Kênh này mình xem hoài luôn!', now()
  FROM seed_videos v
  JOIN users u ON u.id = target_id
  WHERE v.creator_id = fav_gaming
  LIMIT 1;

  RAISE NOTICE 'seed_demo_data: seeded favorite channels for %', current_setting('pipevid.seed_target_email', true);
END $$;

COMMIT;
