-- Migration: 0002_tasks_jobs_nodes.sql
-- Creates tasks, jobs, and nodes tables.
-- See docs/tech_specs/postgres_schema.md#tasks-jobs-and-nodes

-- Nodes table
CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    node_slug TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'registered',
    capability_hash TEXT,
    config_version TEXT,
    last_seen_at TIMESTAMPTZ,
    last_capability_at TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_nodes_node_slug ON nodes(node_slug);
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_last_seen_at ON nodes(last_seen_at);

-- Node capabilities table
CREATE TABLE IF NOT EXISTS node_capabilities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    reported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    capability_snapshot JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_node_capabilities_node_id ON node_capabilities(node_id);
CREATE INDEX IF NOT EXISTS idx_node_capabilities_reported_at ON node_capabilities(reported_at);

-- Tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    project_id UUID,
    status TEXT NOT NULL DEFAULT 'pending',
    prompt TEXT,
    acceptance_criteria JSONB,
    summary TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_created_by ON tasks(created_by);
CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);

-- Jobs table
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    node_id UUID REFERENCES nodes(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    payload JSONB,
    result JSONB,
    lease_id UUID,
    lease_expires_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jobs_task_id ON jobs(task_id);
CREATE INDEX IF NOT EXISTS idx_jobs_node_id ON jobs(node_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_lease_id ON jobs(lease_id) WHERE lease_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_lease_expires_at ON jobs(lease_expires_at) WHERE lease_expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);
