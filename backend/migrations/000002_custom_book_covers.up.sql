ALTER TABLE books
  ADD COLUMN custom_cover_bucket text NOT NULL DEFAULT '',
  ADD COLUMN custom_cover_key text NOT NULL DEFAULT '';
