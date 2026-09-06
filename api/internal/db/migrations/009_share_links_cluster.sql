-- Bind share links to their originating cluster so tokens cannot resolve
-- or start same-named servers in other registered clusters.
ALTER TABLE share_links ADD COLUMN cluster TEXT NOT NULL DEFAULT 'local';
CREATE INDEX idx_share_links_cluster_server ON share_links(cluster, namespace, server_name);
