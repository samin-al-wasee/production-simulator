-- User-owned sandboxes and per-user learning progress (Phase 13.1c). A saved
-- sandbox is the replayable record (ruleset, seed, command log, tick); a game
-- stays a function of that triple, never a tick stream (ADR-0030).

CREATE TABLE sandboxes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text NOT NULL DEFAULT '',
    ruleset    text NOT NULL,
    seed       bigint NOT NULL,
    tick       integer NOT NULL DEFAULT 0,
    save       jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sandboxes_user_id_idx ON sandboxes (user_id, updated_at DESC);

CREATE TABLE learning_progress (
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    exercise_id  text NOT NULL,
    completed_at timestamptz NOT NULL,
    completed_by text NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, exercise_id)
);
