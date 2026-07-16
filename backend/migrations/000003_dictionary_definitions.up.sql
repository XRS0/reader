DROP INDEX IF EXISTS dictionary_search_idx;
CREATE INDEX dictionary_search_idx ON dictionary_entries USING gin(
  to_tsvector(
    'simple',
    coalesce(original_word, '') || ' ' ||
    coalesce(translation, '') || ' ' ||
    coalesce(definition, '') || ' ' ||
    coalesce(lemma, '') || ' ' ||
    coalesce(note, '')
  )
);
