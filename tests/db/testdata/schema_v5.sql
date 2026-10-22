-- Frozen SQLite schema after migration 5, before scheduling and access tables.
-- Keep independent of current Go models and production migration functions.
CREATE TABLE schema_migrations (version integer PRIMARY KEY, applied_at datetime NOT NULL);
INSERT INTO schema_migrations VALUES
    (1, '2026-07-03 00:00:00'), (2, '2026-07-03 00:00:00'),
    (3, '2026-07-03 00:00:00'), (4, '2026-07-03 00:00:00'),
    (5, '2026-07-03 00:00:00');

CREATE TABLE users (
    id integer PRIMARY KEY AUTOINCREMENT, email text NOT NULL,
    password_hash text NOT NULL, name text, role text NOT NULL DEFAULT 'user',
    created_at datetime, updated_at datetime
);
CREATE UNIQUE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

CREATE TABLE sessions (
    id integer PRIMARY KEY AUTOINCREMENT, user_id integer NOT NULL,
    token_hash text NOT NULL, expires_at datetime NOT NULL, created_at datetime,
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id)
        ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE UNIQUE INDEX idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE contests (
    id integer PRIMARY KEY AUTOINCREMENT, title text NOT NULL,
    description text, visibility text NOT NULL DEFAULT 'draft',
    start_at datetime, end_at datetime, created_at datetime, updated_at datetime,
    CONSTRAINT chk_contests_title CHECK (title <> ''),
    CONSTRAINT chk_contests_visibility CHECK (visibility IN ('draft','published')),
    CONSTRAINT contest_dates CHECK (end_at IS NULL OR start_at IS NULL OR end_at > start_at)
);
CREATE INDEX idx_contests_visibility ON contests(visibility);

CREATE TABLE series (
    id integer PRIMARY KEY AUTOINCREMENT, contest_id integer NOT NULL,
    title text NOT NULL, position integer NOT NULL, created_at datetime, updated_at datetime,
    CONSTRAINT chk_series_title CHECK (title <> ''),
    CONSTRAINT chk_series_position CHECK (position >= 0),
    CONSTRAINT fk_series_contest FOREIGN KEY (contest_id) REFERENCES contests(id)
        ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX idx_series_contest_id ON series(contest_id);
CREATE UNIQUE INDEX idx_series_contest_position ON series(contest_id, position);

CREATE TABLE artifacts (
    id integer PRIMARY KEY AUTOINCREMENT, owner_id integer NOT NULL,
    storage_key text NOT NULL, sha256 text NOT NULL, size_bytes integer NOT NULL,
    content_type text NOT NULL, filename text NOT NULL, created_at datetime,
    CONSTRAINT chk_artifacts_size_bytes CHECK (size_bytes >= 0),
    CONSTRAINT fk_artifacts_owner FOREIGN KEY (owner_id) REFERENCES users(id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);
CREATE INDEX idx_artifacts_owner_id ON artifacts(owner_id);
CREATE UNIQUE INDEX idx_artifacts_storage_key ON artifacts(storage_key);
CREATE INDEX idx_artifacts_sha256 ON artifacts(sha256);

CREATE TABLE problems (
    id integer PRIMARY KEY AUTOINCREMENT, owner_id integer NOT NULL,
    title text NOT NULL, code text NOT NULL, kind text NOT NULL DEFAULT 'programming',
    created_at datetime, updated_at datetime,
    CONSTRAINT chk_problems_title CHECK (title <> ''),
    CONSTRAINT chk_problems_code CHECK (code <> ''),
    CONSTRAINT chk_problems_kind CHECK (kind = 'programming'),
    CONSTRAINT fk_problems_owner FOREIGN KEY (owner_id) REFERENCES users(id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);
CREATE INDEX idx_problems_owner_id ON problems(owner_id);
CREATE UNIQUE INDEX idx_problems_code ON problems(code);

CREATE TABLE problem_revisions (
    id integer PRIMARY KEY AUTOINCREMENT, problem_id integer NOT NULL,
    number integer NOT NULL, state text NOT NULL DEFAULT 'draft', statement text NOT NULL,
    pdf_artifact_id integer, time_limit_millis integer, memory_limit_kib integer,
    language_metadata text NOT NULL DEFAULT '[]', checker_metadata text NOT NULL DEFAULT '',
    published_at datetime, created_at datetime, updated_at datetime,
    CONSTRAINT chk_problem_revisions_number CHECK (number > 0),
    CONSTRAINT chk_problem_revisions_state CHECK (state IN ('draft','published')),
    CONSTRAINT chk_problem_revisions_time_limit_millis CHECK (time_limit_millis IS NULL OR time_limit_millis > 0),
    CONSTRAINT chk_problem_revisions_memory_limit_ki_b CHECK (memory_limit_kib IS NULL OR memory_limit_kib > 0),
    CONSTRAINT fk_problems_revisions FOREIGN KEY (problem_id) REFERENCES problems(id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_problem_revisions_pdf_artifact FOREIGN KEY (pdf_artifact_id) REFERENCES artifacts(id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);
CREATE UNIQUE INDEX idx_revision_number ON problem_revisions(problem_id, number);
CREATE INDEX idx_problem_revisions_state ON problem_revisions(state);
CREATE INDEX idx_problem_revisions_pdf_artifact_id ON problem_revisions(pdf_artifact_id);

CREATE TABLE series_problems (
    id integer PRIMARY KEY AUTOINCREMENT, series_id integer NOT NULL,
    problem_id integer NOT NULL, published_revision_id integer NOT NULL,
    code text NOT NULL, position integer NOT NULL, created_at datetime, updated_at datetime,
    CONSTRAINT chk_series_problems_code CHECK (code <> ''),
    CONSTRAINT chk_series_problems_position CHECK (position >= 0),
    CONSTRAINT fk_series_problems_series FOREIGN KEY (series_id) REFERENCES series(id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_series_problems_problem FOREIGN KEY (problem_id) REFERENCES problems(id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_series_problems_published_revision FOREIGN KEY (published_revision_id) REFERENCES problem_revisions(id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);
CREATE UNIQUE INDEX idx_series_problem_position ON series_problems(series_id, position);
CREATE INDEX idx_series_problems_problem_id ON series_problems(problem_id);
CREATE UNIQUE INDEX idx_series_problem_code ON series_problems(code);

CREATE TABLE test_cases (
    id integer PRIMARY KEY AUTOINCREMENT, problem_revision_id integer NOT NULL,
    position integer NOT NULL, is_sample numeric NOT NULL DEFAULT false,
    points integer, time_limit_override_millis integer, input_artifact_id integer NOT NULL,
    output_artifact_id integer NOT NULL, created_at datetime, updated_at datetime,
    CONSTRAINT chk_test_cases_position CHECK (position >= 0),
    CONSTRAINT chk_test_cases_points CHECK (points IS NULL OR points >= 0),
    CONSTRAINT chk_test_cases_time_limit_override_millis CHECK (time_limit_override_millis IS NULL OR time_limit_override_millis > 0),
    CONSTRAINT fk_problem_revisions_tests FOREIGN KEY (problem_revision_id) REFERENCES problem_revisions(id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_test_cases_input_artifact FOREIGN KEY (input_artifact_id) REFERENCES artifacts(id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_test_cases_output_artifact FOREIGN KEY (output_artifact_id) REFERENCES artifacts(id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);
CREATE UNIQUE INDEX idx_test_revision_position ON test_cases(problem_revision_id, position);
