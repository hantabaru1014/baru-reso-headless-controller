CREATE TABLE resonite_versions (
    manifest_id TEXT NOT NULL,
    branch TEXT NOT NULL, -- 'public' / 'prerelease' / 'release' / 'headless' / 'pre-splittening'
    game_version TEXT, -- versions.json の gameVersion (null あり)
    released_at TIMESTAMP WITH TIME ZONE NOT NULL,
    build_status TEXT NOT NULL DEFAULT 'not_built', -- 'not_built' / 'building' / 'built' / 'failed'
    image_tag TEXT, -- built のときに set
    built_with_app_version TEXT, -- container repo の Headless/AppVersion (built のときに set)
    built_at TIMESTAMP WITH TIME ZONE,
    build_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (manifest_id, branch)
);

CREATE INDEX idx_resonite_versions_branch_released
    ON resonite_versions (branch, released_at DESC);
CREATE INDEX idx_resonite_versions_image_tag
    ON resonite_versions (image_tag) WHERE image_tag IS NOT NULL;

CREATE TRIGGER update_resonite_versions_modtime
BEFORE UPDATE ON resonite_versions
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();
