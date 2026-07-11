CREATE TABLE folders (
    id TEXT PRIMARY KEY,
    parent_id TEXT REFERENCES folders (id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX folders_parent_id_idx ON folders (parent_id);

CREATE UNIQUE INDEX folders_owner_parent_name_idx
    ON folders (owner_id, COALESCE(parent_id, ''), name);

CREATE TABLE assets (
    id TEXT PRIMARY KEY,
    folder_id TEXT REFERENCES folders (id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL,
    name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size BIGINT NOT NULL,
    checksum TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX assets_folder_id_idx ON assets (folder_id);

CREATE UNIQUE INDEX assets_owner_folder_name_idx
    ON assets (owner_id, COALESCE(folder_id, ''), name);

CREATE INDEX assets_tags_idx ON assets USING GIN (tags);
