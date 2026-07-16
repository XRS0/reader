CREATE TABLE users (
  id uuid PRIMARY KEY,
  email text NOT NULL,
  password_hash text NOT NULL,
  display_name text NOT NULL DEFAULT '',
  locale varchar(16) NOT NULL DEFAULT 'en',
  timezone text NOT NULL DEFAULT 'UTC',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX users_email_unique_active ON users (lower(email)) WHERE deleted_at IS NULL;

CREATE TABLE devices (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_key text NOT NULL,
  name text NOT NULL DEFAULT '',
  user_agent text NOT NULL DEFAULT '',
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, device_key)
);
CREATE INDEX devices_user_last_seen_idx ON devices(user_id, last_seen_at DESC);

CREATE TABLE refresh_tokens (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  family_id uuid NOT NULL,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  replaced_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz
);
CREATE INDEX refresh_tokens_user_active_idx ON refresh_tokens(user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens(family_id);

CREATE TABLE user_sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id uuid REFERENCES devices(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  ended_at timestamptz
);

CREATE TABLE books (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title text NOT NULL,
  author text NOT NULL DEFAULT '',
  language varchar(32) NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  format varchar(16) NOT NULL CHECK (format IN ('txt','fb2','epub')),
  status varchar(16) NOT NULL CHECK (status IN ('uploaded','queued','processing','ready','failed')),
  sha256 char(64) NOT NULL,
  original_filename text NOT NULL DEFAULT '',
  original_mime text NOT NULL DEFAULT '',
  original_size bigint NOT NULL CHECK (original_size >= 0),
  original_bucket text NOT NULL,
  original_key text NOT NULL,
  cover_bucket text NOT NULL DEFAULT '',
  cover_key text NOT NULL DEFAULT '',
  processing_version integer NOT NULL DEFAULT 1 CHECK (processing_version > 0),
  processing_error text NOT NULL DEFAULT '',
  is_favorite boolean NOT NULL DEFAULT false,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX books_user_sha_active_unique ON books(user_id, sha256) WHERE deleted_at IS NULL;
CREATE INDEX books_user_created_idx ON books(user_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX books_user_status_idx ON books(user_id, status) WHERE deleted_at IS NULL;
CREATE INDEX books_search_idx ON books USING gin(to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(author,'') || ' ' || coalesce(description,'')));
CREATE TABLE book_tags (
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  tag text NOT NULL CHECK(length(tag) BETWEEN 1 AND 50),
  PRIMARY KEY(book_id, tag)
);
CREATE INDEX book_tags_tag_idx ON book_tags(tag, book_id);

CREATE TABLE authors (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  normalized_name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(normalized_name)
);
CREATE TABLE book_authors (
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  author_id uuid NOT NULL REFERENCES authors(id) ON DELETE RESTRICT,
  ordinal integer NOT NULL DEFAULT 0,
  PRIMARY KEY(book_id, author_id)
);

CREATE TABLE book_files (
  id uuid PRIMARY KEY,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  kind text NOT NULL,
  bucket text NOT NULL,
  object_key text NOT NULL,
  sha256 char(64) NOT NULL,
  size bigint NOT NULL CHECK(size >= 0),
  media_type text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(book_id, kind, sha256)
);

CREATE TABLE book_chapters (
  id uuid PRIMARY KEY,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  version integer NOT NULL CHECK(version > 0),
  ordinal integer NOT NULL CHECK(ordinal >= 0),
  title text NOT NULL DEFAULT '',
  source_ref text NOT NULL DEFAULT '',
  content_html text NOT NULL DEFAULT '',
  content_text text NOT NULL DEFAULT '',
  content_bucket text NOT NULL DEFAULT '',
  content_key text NOT NULL DEFAULT '',
  character_count integer NOT NULL DEFAULT 0 CHECK(character_count >= 0),
  word_count integer NOT NULL DEFAULT 0 CHECK(word_count >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(book_id, version, ordinal)
);
CREATE INDEX book_chapters_book_idx ON book_chapters(book_id, version, ordinal);
CREATE INDEX book_chapters_search_idx ON book_chapters USING gin(to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(content_text,'')));

CREATE TABLE book_assets (
  id uuid PRIMARY KEY,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  version integer NOT NULL,
  source_ref text NOT NULL,
  media_type text NOT NULL,
  bucket text NOT NULL,
  object_key text NOT NULL,
  size bigint NOT NULL CHECK(size >= 0),
  sha256 char(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(book_id, version, source_ref)
);

CREATE TABLE book_processing_jobs (
  id uuid PRIMARY KEY,
  type text NOT NULL,
  payload jsonb NOT NULL,
  status varchar(16) NOT NULL CHECK(status IN ('queued','running','retry','completed','dead')),
  priority integer NOT NULL DEFAULT 0,
  attempts integer NOT NULL DEFAULT 0 CHECK(attempts >= 0),
  max_attempts integer NOT NULL DEFAULT 5 CHECK(max_attempts > 0),
  run_after timestamptz NOT NULL DEFAULT now(),
  locked_at timestamptz,
  locked_by text NOT NULL DEFAULT '',
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);
CREATE INDEX jobs_claim_idx ON book_processing_jobs(priority DESC, run_after, created_at) WHERE status IN ('queued','retry');
CREATE INDEX jobs_running_idx ON book_processing_jobs(locked_at) WHERE status = 'running';

CREATE TABLE reading_progress (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  chapter_id uuid REFERENCES book_chapters(id) ON DELETE SET NULL,
  locator_type text NOT NULL,
  locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  character_offset bigint NOT NULL DEFAULT 0 CHECK(character_offset >= 0),
  text_anchor varchar(512) NOT NULL DEFAULT '',
  chapter_progress double precision NOT NULL DEFAULT 0 CHECK(chapter_progress BETWEEN 0 AND 100),
  progress_percent double precision NOT NULL DEFAULT 0 CHECK(progress_percent BETWEEN 0 AND 100),
  scroll_percent double precision NOT NULL DEFAULT 0 CHECK(scroll_percent BETWEEN 0 AND 100),
  revision bigint NOT NULL DEFAULT 1 CHECK(revision > 0),
  client_id text NOT NULL DEFAULT '',
  device_id uuid REFERENCES devices(id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(user_id, book_id)
);
CREATE INDEX reading_progress_recent_idx ON reading_progress(user_id, updated_at DESC);

CREATE TABLE reading_sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  device_id uuid REFERENCES devices(id) ON DELETE SET NULL,
  started_at timestamptz NOT NULL,
  last_activity_at timestamptz NOT NULL,
  last_heartbeat_at timestamptz NOT NULL,
  ended_at timestamptz,
  active_seconds bigint NOT NULL DEFAULT 0 CHECK(active_seconds >= 0),
  idle_seconds bigint NOT NULL DEFAULT 0 CHECK(idle_seconds >= 0),
  start_locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  end_locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  start_progress_percent double precision NOT NULL DEFAULT 0 CHECK(start_progress_percent BETWEEN 0 AND 100),
  end_progress_percent double precision NOT NULL DEFAULT 0 CHECK(end_progress_percent BETWEEN 0 AND 100),
  characters_read bigint NOT NULL DEFAULT 0 CHECK(characters_read >= 0),
  words_read_estimate bigint NOT NULL DEFAULT 0 CHECK(words_read_estimate >= 0),
  pages_read_estimate double precision NOT NULL DEFAULT 0 CHECK(pages_read_estimate >= 0),
  close_reason text NOT NULL DEFAULT '',
  status varchar(16) NOT NULL CHECK(status IN ('active','idle','finished','stale','finalized')),
  last_sequence bigint NOT NULL DEFAULT 0,
  last_was_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reading_sessions_user_started_idx ON reading_sessions(user_id, started_at DESC);
CREATE INDEX reading_sessions_active_idx ON reading_sessions(last_heartbeat_at) WHERE status IN ('active','idle');

CREATE TABLE reading_events (
  id uuid PRIMARY KEY,
  session_id uuid NOT NULL REFERENCES reading_sessions(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  type text NOT NULL,
  occurred_at timestamptz NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  idempotency_key text NOT NULL,
  sequence_number bigint NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE(session_id, idempotency_key)
);
CREATE INDEX reading_events_session_seq_idx ON reading_events(session_id, sequence_number);
CREATE INDEX reading_events_user_received_idx ON reading_events(user_id, received_at DESC);

CREATE TABLE reader_preferences (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  theme text NOT NULL DEFAULT 'light' CHECK(theme IN ('light','warm','sepia','dark','custom')),
  background_color varchar(16) NOT NULL DEFAULT '#ffffff',
  text_color varchar(16) NOT NULL DEFAULT '#252525',
  accent_color varchar(16) NOT NULL DEFAULT '#3468c0',
  font_family text NOT NULL DEFAULT 'system',
  font_size double precision NOT NULL DEFAULT 18 CHECK(font_size BETWEEN 10 AND 48),
  font_weight integer NOT NULL DEFAULT 400 CHECK(font_weight BETWEEN 100 AND 900),
  line_height double precision NOT NULL DEFAULT 1.6 CHECK(line_height BETWEEN 1 AND 3),
  letter_spacing double precision NOT NULL DEFAULT 0 CHECK(letter_spacing BETWEEN -2 AND 10),
  content_width integer NOT NULL DEFAULT 720 CHECK(content_width BETWEEN 320 AND 1400),
  margin_size integer NOT NULL DEFAULT 32 CHECK(margin_size BETWEEN 0 AND 200),
  text_align text NOT NULL DEFAULT 'left' CHECK(text_align IN ('left','justify')),
  navigation_mode text NOT NULL DEFAULT 'scroll' CHECK(navigation_mode IN ('scroll','paged')),
  show_progress boolean NOT NULL DEFAULT true,
  show_remaining_time boolean NOT NULL DEFAULT true,
  ui_intensity double precision NOT NULL DEFAULT 0.8 CHECK(ui_intensity BETWEEN 0 AND 1),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE book_reader_preferences (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  theme text NOT NULL DEFAULT 'light' CHECK(theme IN ('light','warm','sepia','dark','custom')),
  background_color varchar(16) NOT NULL DEFAULT '#ffffff', text_color varchar(16) NOT NULL DEFAULT '#252525', accent_color varchar(16) NOT NULL DEFAULT '#3468c0',
  font_family text NOT NULL DEFAULT 'system', font_size double precision NOT NULL DEFAULT 18 CHECK(font_size BETWEEN 10 AND 48),
  font_weight integer NOT NULL DEFAULT 400 CHECK(font_weight BETWEEN 100 AND 900), line_height double precision NOT NULL DEFAULT 1.6 CHECK(line_height BETWEEN 1 AND 3),
  letter_spacing double precision NOT NULL DEFAULT 0 CHECK(letter_spacing BETWEEN -2 AND 10), content_width integer NOT NULL DEFAULT 720 CHECK(content_width BETWEEN 320 AND 1400),
  margin_size integer NOT NULL DEFAULT 32 CHECK(margin_size BETWEEN 0 AND 200), text_align text NOT NULL DEFAULT 'left' CHECK(text_align IN ('left','justify')),
  navigation_mode text NOT NULL DEFAULT 'scroll' CHECK(navigation_mode IN ('scroll','paged')),
  show_progress boolean NOT NULL DEFAULT true, show_remaining_time boolean NOT NULL DEFAULT true,
  ui_intensity double precision NOT NULL DEFAULT 0.8 CHECK(ui_intensity BETWEEN 0 AND 1), updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, book_id)
);

CREATE TABLE bookmarks (
  id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE, chapter_id uuid REFERENCES book_chapters(id) ON DELETE SET NULL,
  locator jsonb NOT NULL, progress_percent double precision NOT NULL CHECK(progress_percent BETWEEN 0 AND 100),
  title text NOT NULL DEFAULT '', note text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bookmarks_user_book_idx ON bookmarks(user_id, book_id, created_at DESC);

CREATE TABLE highlights (
  id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE, chapter_id uuid REFERENCES book_chapters(id) ON DELETE SET NULL,
  locator jsonb NOT NULL, text_anchor varchar(512) NOT NULL DEFAULT '', selected_text text NOT NULL, context text NOT NULL DEFAULT '',
  color varchar(16) NOT NULL DEFAULT 'yellow', note text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK(length(selected_text) <= 20000)
);
CREATE INDEX highlights_user_book_idx ON highlights(user_id, book_id, created_at DESC);

CREATE TABLE notes (
  id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  book_id uuid REFERENCES books(id) ON DELETE CASCADE, highlight_id uuid REFERENCES highlights(id) ON DELETE SET NULL,
  title text NOT NULL DEFAULT '', schema_version integer NOT NULL DEFAULT 1, blocks jsonb NOT NULL DEFAULT '[]'::jsonb,
  search_text text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz
);
CREATE INDEX notes_user_updated_idx ON notes(user_id, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX notes_search_idx ON notes USING gin(to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(search_text,'')));
CREATE TABLE note_blocks (
  id uuid PRIMARY KEY, note_id uuid NOT NULL REFERENCES notes(id) ON DELETE CASCADE, ordinal integer NOT NULL,
  type text NOT NULL, data jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(note_id, ordinal)
);

CREATE TABLE translation_cache (
  id uuid PRIMARY KEY, cache_key char(64) NOT NULL UNIQUE, request_type text NOT NULL,
  source_language varchar(32) NOT NULL, target_language varchar(32) NOT NULL, normalized_text text NOT NULL,
  provider text NOT NULL, provider_model text NOT NULL, prompt_version text NOT NULL,
  result jsonb NOT NULL, result_version integer NOT NULL DEFAULT 1, use_count bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(), last_used_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL, invalidated_at timestamptz
);
CREATE INDEX translation_cache_expiry_idx ON translation_cache(expires_at) WHERE invalidated_at IS NULL;

CREATE TABLE dictionary_entries (
  id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_language varchar(32) NOT NULL, target_language varchar(32) NOT NULL,
  original_word text NOT NULL, normalized_word text NOT NULL, lemma text NOT NULL DEFAULT '', transcription text NOT NULL DEFAULT '',
  part_of_speech text NOT NULL DEFAULT '', translation text NOT NULL, alternative_translations jsonb NOT NULL DEFAULT '[]'::jsonb,
  definition text NOT NULL DEFAULT '', note text NOT NULL DEFAULT '', status varchar(16) NOT NULL DEFAULT 'unknown' CHECK(status IN ('unknown','learning','known','mastered','ignored')),
  encounter_count integer NOT NULL DEFAULT 1 CHECK(encounter_count > 0), first_seen_at timestamptz NOT NULL DEFAULT now(), last_seen_at timestamptz NOT NULL DEFAULT now(),
  next_review_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
  UNIQUE(user_id, source_language, target_language, normalized_word)
);
CREATE INDEX dictionary_user_status_idx ON dictionary_entries(user_id, status, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX dictionary_search_idx ON dictionary_entries USING gin(to_tsvector('simple', coalesce(original_word,'') || ' ' || coalesce(translation,'') || ' ' || coalesce(lemma,'')));

CREATE TABLE word_occurrences (
  id uuid PRIMARY KEY, dictionary_entry_id uuid NOT NULL REFERENCES dictionary_entries(id) ON DELETE CASCADE,
  book_id uuid REFERENCES books(id) ON DELETE CASCADE, chapter_id uuid REFERENCES book_chapters(id) ON DELETE SET NULL,
  locator jsonb NOT NULL DEFAULT '{}'::jsonb, sentence text NOT NULL DEFAULT '', context_before text NOT NULL DEFAULT '', context_after text NOT NULL DEFAULT '',
  encountered_at timestamptz NOT NULL DEFAULT now(), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX occurrences_entry_time_idx ON word_occurrences(dictionary_entry_id, encountered_at DESC);

CREATE TABLE daily_reading_stats (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, local_date date NOT NULL, timezone text NOT NULL,
  active_seconds bigint NOT NULL DEFAULT 0, idle_seconds bigint NOT NULL DEFAULT 0, session_count integer NOT NULL DEFAULT 0,
  words_read bigint NOT NULL DEFAULT 0, pages_read double precision NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, local_date, timezone)
);
CREATE TABLE book_reading_stats (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  active_seconds bigint NOT NULL DEFAULT 0, idle_seconds bigint NOT NULL DEFAULT 0, session_count integer NOT NULL DEFAULT 0,
  words_read bigint NOT NULL DEFAULT 0, pages_read double precision NOT NULL DEFAULT 0, last_read_at timestamptz, updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, book_id)
);

CREATE TABLE idempotency_keys (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, scope text NOT NULL, key text NOT NULL,
  request_hash char(64) NOT NULL, response_status integer, response_body jsonb, expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, scope, key)
);
CREATE TABLE audit_logs (
  id uuid PRIMARY KEY, user_id uuid REFERENCES users(id) ON DELETE SET NULL, action text NOT NULL, resource_type text NOT NULL,
  resource_id uuid, request_id text NOT NULL DEFAULT '', ip inet, metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_user_time_idx ON audit_logs(user_id, created_at DESC);
