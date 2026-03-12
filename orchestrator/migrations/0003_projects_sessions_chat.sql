-- Migration: 0003_projects_sessions_chat.sql
-- Creates projects, sessions, chat_threads, chat_messages, chat_audit_log.
-- See docs/tech_specs/postgres_schema.md (Projects, Runs and Sessions, Chat Threads and Messages, Chat Audit Log).

-- Projects table (required for chat_threads.project_id and chat_audit_log.project_id)
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug);
CREATE INDEX IF NOT EXISTS idx_projects_is_active ON projects(is_active);

-- Sessions table (required for chat_threads.session_id)
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_parent_session_id ON sessions(parent_session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);

-- Chat threads table
CREATE TABLE IF NOT EXISTS chat_threads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    title TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_threads_user_id_updated_at ON chat_threads(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_chat_threads_project_id_updated_at ON chat_threads(project_id, updated_at) WHERE project_id IS NOT NULL;

-- Chat messages table
CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    thread_id UUID NOT NULL REFERENCES chat_threads(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_thread_id_created_at ON chat_messages(thread_id, created_at);

-- Chat audit log table (redaction and outcome per OpenAI chat API spec)
CREATE TABLE IF NOT EXISTS chat_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    outcome TEXT NOT NULL,
    error_code TEXT,
    redaction_applied BOOLEAN NOT NULL DEFAULT false,
    redaction_kinds JSONB,
    duration_ms INTEGER,
    request_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_chat_audit_log_created_at ON chat_audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_chat_audit_log_user_id ON chat_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_audit_log_project_id ON chat_audit_log(project_id) WHERE project_id IS NOT NULL;
