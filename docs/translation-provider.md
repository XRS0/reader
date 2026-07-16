# Translation provider and cache

> **Document type: target provider/privacy contract.** Mock/provider/cache service code may precede HTTP, persistence, circuit-breaker and dictionary transaction wiring. See [implementation-plan.md](implementation-plan.md).

## Port and adapters

Business use cases depend on a `TranslationProvider` port with word translation, text translation, and language detection operations. Adapters include a deterministic mock for local development/tests and an OpenAI adapter when configured. DeepL, Google Translate, or a local model can be introduced as adapters without changing dictionary or HTTP contracts.

Provider configuration is validated at process start. `TRANSLATION_PROVIDER=mock` requires no network or API key. An external adapter must have a request deadline, bounded retry with exponential backoff/jitter, circuit breaker, response schema validation, and metrics that never label full input text.

## Privacy boundary

Send only the selected word/phrase and bounded surrounding context (normally one nearby sentence), plus source/target language and necessary preference flags. Never send a whole chapter/book, user email, original object key, note collection, auth token, or unnecessary locator data. Structured logs contain request/trace IDs, lengths, language/provider, latency, result and a one-way cache hash—not the complete selection or provider credentials.

## Request flow

```mermaid
sequenceDiagram
    actor U as Reader
    participant API
    participant Cache as PostgreSQL cache
    participant P as Translation provider
    U->>API: selection + bounded context
    API->>API: ownership, length, language validation; normalize
    API->>Cache: lookup versioned cache hash
    alt fresh cached result
      Cache-->>API: result
      API-->>U: translation, cache_hit=true
    else miss
      API->>P: minimal provider request with deadline
      P-->>API: typed result
      API->>API: validate and strip unsafe/unexpected content
      API->>Cache: upsert result + TTL/use counters
      API-->>U: translation, cache_hit=false
    end
```

The cache key hashes a canonical serialization of normalized text, source language, target language, request type (`word`/`text`/`detect`), provider, model, prompt version, and result-schema version. Unicode normalization and whitespace/case policy must be language-aware and stable. A key collision is still guarded by storing non-sensitive canonical dimensions. TTL, `last_used_at`, use count, and manual invalidation support cost control and provider/prompt upgrades.

## Word result

A word result can include normalized form, lemma, translation, transcription, part of speech, short definition, alternatives, one usage example, detected language, and confidence when supported. Missing optional linguistic fields are not fabricated. A text result contains original selection, translation, detected language, and only necessary short explanation.

## Dictionary transaction

```mermaid
sequenceDiagram
    actor U as User
    participant API
    participant DB as PostgreSQL
    U->>API: Add translated word + occurrence + Idempotency-Key
    API->>DB: BEGIN; claim key
    API->>DB: upsert active entry by user/languages/normalized word
    alt entry existed
      API->>DB: update last_seen_at; increment encounter_count
    else new entry
      API->>DB: create with status unknown/learning
    end
    API->>DB: insert deduplicated occurrence with book/chapter ownership
    API->>DB: COMMIT
    API-->>U: one entry with complete encounter count
```

Repeat encounters create occurrences but never duplicate the primary entry. The user can edit provider-derived fields; a later cached/provider response must not silently overwrite user edits. Manual mode is default. Assisted mode may propose candidates, but it requires confirmation and does not claim to know whether a user knows a word.

## Abuse and failure handling

Rate limit per authenticated user and defensively per network/IP at the edge; cap selected and context lengths before cache/provider access. Only retry timeout, rate-limit, and documented transient provider failures. Honor cancellation. Circuit-open/provider errors use a stable public error code and `Retry-After`, without leaking a provider response. Long permitted fragments may become a PostgreSQL job, but the default interactive word path remains synchronous.
