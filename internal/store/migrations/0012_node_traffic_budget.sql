ALTER TABLE nodes ADD COLUMN traffic_limit_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_limit_bytes >= 0);
ALTER TABLE nodes ADD COLUMN traffic_reset_day INTEGER NOT NULL DEFAULT 1
    CHECK (traffic_reset_day BETWEEN 1 AND 31);
ALTER TABLE nodes ADD COLUMN traffic_cycle_started_at INTEGER;
ALTER TABLE nodes ADD COLUMN traffic_cycle_upload_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_cycle_upload_bytes >= 0);
ALTER TABLE nodes ADD COLUMN traffic_cycle_download_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_cycle_download_bytes >= 0);
ALTER TABLE nodes ADD COLUMN traffic_calibration_bytes INTEGER
    CHECK (traffic_calibration_bytes IS NULL OR traffic_calibration_bytes >= 0);
ALTER TABLE nodes ADD COLUMN traffic_calibration_proxy_bytes INTEGER
    CHECK (traffic_calibration_proxy_bytes IS NULL OR traffic_calibration_proxy_bytes >= 0);
ALTER TABLE nodes ADD COLUMN traffic_calibrated_at INTEGER;

UPDATE nodes
SET traffic_cycle_started_at = unixepoch('now', 'start of month') * 1000,
    traffic_cycle_upload_bytes = COALESCE((
        SELECT SUM(upload_bytes) FROM traffic_batches
        WHERE traffic_batches.node_id = nodes.id
          AND sampled_at >= unixepoch('now', 'start of month') * 1000
    ), 0),
    traffic_cycle_download_bytes = COALESCE((
        SELECT SUM(download_bytes) FROM traffic_batches
        WHERE traffic_batches.node_id = nodes.id
          AND sampled_at >= unixepoch('now', 'start of month') * 1000
    ), 0);
