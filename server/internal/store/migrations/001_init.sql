CREATE TABLE users (
  id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  display_name_key TEXT NOT NULL UNIQUE,
  passphrase_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE identity_records (
  user_id TEXT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  legal_name TEXT NOT NULL,
  date_of_birth DATE NOT NULL,
  document_type TEXT NOT NULL CHECK (document_type IN ('passport', 'national_id', 'drivers_license')),
  document_number_hash TEXT NOT NULL UNIQUE,
  document_last4 TEXT NOT NULL,
  photo_path TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status = 'id_on_file'),
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE circles (
  user_id TEXT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  supervisor_limit INT NOT NULL CHECK (supervisor_limit BETWEEN 1 AND 5),
  sharing_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE invites (
  code TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  used_by TEXT REFERENCES users (id)
);

CREATE TABLE supervisor_links (
  id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  supervisor_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('active', 'revoked')),
  created_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  UNIQUE (person_id, supervisor_id)
);

CREATE TABLE places (
  id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  label TEXT NOT NULL,
  latitude DOUBLE PRECISION NOT NULL,
  longitude DOUBLE PRECISION NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE location_points (
  id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  latitude DOUBLE PRECISION NOT NULL,
  longitude DOUBLE PRECISION NOT NULL,
  accuracy_m DOUBLE PRECISION NOT NULL,
  recorded_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX location_points_person_recorded ON location_points (person_id, recorded_at DESC);

CREATE TABLE safety_checks (
  id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  started_by TEXT NOT NULL REFERENCES users (id),
  reason TEXT NOT NULL CHECK (reason IN ('not_responding', 'unwell', 'self_harm', 'missing')),
  status TEXT NOT NULL CHECK (status IN ('pending', 'confirmed_safe', 'unanswered')),
  started_at TIMESTAMPTZ NOT NULL,
  respond_by TIMESTAMPTZ NOT NULL,
  resolved_at TIMESTAMPTZ,
  confirmed_by TEXT REFERENCES users (id)
);

CREATE INDEX safety_checks_person_started ON safety_checks (person_id, started_at DESC);

CREATE UNIQUE INDEX safety_checks_one_pending ON safety_checks (person_id) WHERE status = 'pending';
