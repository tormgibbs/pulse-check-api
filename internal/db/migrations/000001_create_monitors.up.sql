CREATE TABLE monitors (
  id                  TEXT PRIMARY KEY,
  timeout_seconds     INTEGER NOT NULL,
  failure_threshold   INTEGER NOT NULL DEFAULT 1,
  failure_count       INTEGER NOT NULL DEFAULT 0,
  recovery_threshold  INTEGER NOT NULL DEFAULT 3,
  recovery_window     INTEGER NOT NULL,
  consecutive_hb      INTEGER NOT NULL DEFAULT 0,
  alert_on_recovery   BOOLEAN NOT NULL DEFAULT TRUE,
  status              TEXT NOT NULL DEFAULT 'active',
  last_heartbeat_at   TIMESTAMPTZ NOT NULL,
  recovery_deadline   TIMESTAMPTZ,
  alerted_at          TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT valid_status CHECK (status IN ('active', 'paused', 'down', 'recovering')),
  CONSTRAINT valid_timeout CHECK (timeout_seconds >= 10 AND timeout_seconds <= 86400),
  CONSTRAINT valid_failure_threshold CHECK (failure_threshold >= 1),
  CONSTRAINT valid_recovery_threshold CHECK (recovery_threshold >= 1),
  CONSTRAINT valid_recovery_window CHECK (recovery_window >= 1)
);
