-- Initial schema for BPMN engine store

CREATE TABLE IF NOT EXISTS processes (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL DEFAULT 1,
    definition  JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS instances (
    id          TEXT PRIMARY KEY,
    process_id  TEXT NOT NULL REFERENCES processes(id),
    title       TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'CREATED',
    current_user TEXT NOT NULL DEFAULT '',
    variables   JSONB NOT NULL DEFAULT '{}',
    pin         TEXT NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_instances_process_id ON instances(process_id);
CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);

CREATE TABLE IF NOT EXISTS flows (
    id           TEXT PRIMARY KEY,
    instance_id  TEXT NOT NULL REFERENCES instances(id),
    element_id   TEXT NOT NULL,
    element_type TEXT NOT NULL,
    thread_id    INTEGER NOT NULL DEFAULT 1,
    previous_id  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'PENDING',
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    duration_ms  INTEGER
);

CREATE INDEX IF NOT EXISTS idx_flows_instance_id ON flows(instance_id);
CREATE INDEX IF NOT EXISTS idx_flows_thread_id ON flows(instance_id, thread_id);

CREATE TABLE IF NOT EXISTS threads (
    id            SERIAL PRIMARY KEY,
    instance_id   TEXT NOT NULL REFERENCES instances(id),
    thread_index  INTEGER NOT NULL,
    parent_index  INTEGER,
    flow_id       TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(instance_id, thread_index)
);

CREATE INDEX IF NOT EXISTS idx_threads_instance_id ON threads(instance_id);

CREATE TABLE IF NOT EXISTS jobs (
    id           TEXT PRIMARY KEY,
    instance_id  TEXT NOT NULL REFERENCES instances(id),
    flow_id      TEXT NOT NULL DEFAULT '',
    type         TEXT NOT NULL DEFAULT '',
    payload      JSONB NOT NULL DEFAULT '{}',
    status       TEXT NOT NULL DEFAULT 'PENDING',
    retry_count  INTEGER NOT NULL DEFAULT 0,
    max_retries  INTEGER NOT NULL DEFAULT 3,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at  TIMESTAMPTZ,
    error_message TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_scheduled ON jobs(scheduled_at) WHERE status = 'PENDING';

CREATE TABLE IF NOT EXISTS dead_letters (
    id           TEXT PRIMARY KEY,
    job_id       TEXT NOT NULL DEFAULT '',
    instance_id  TEXT NOT NULL REFERENCES instances(id),
    type         TEXT NOT NULL DEFAULT '',
    payload      JSONB NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    retry_count  INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dead_letters_instance_id ON dead_letters(instance_id);

CREATE TABLE IF NOT EXISTS execution_log (
    id           TEXT PRIMARY KEY,
    instance_id  TEXT NOT NULL REFERENCES instances(id),
    element_id   TEXT NOT NULL DEFAULT '',
    element_type TEXT NOT NULL DEFAULT '',
    action       TEXT NOT NULL DEFAULT '',
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_execution_log_instance_id ON execution_log(instance_id);
