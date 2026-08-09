ALTER TABLE nodes ADD COLUMN hostname TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN kernel_version TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN cpu_cores INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN swap_used_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN swap_total_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN disk_read_bytes_per_second INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN disk_write_bytes_per_second INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN network_rx_bytes_total INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN network_tx_bytes_total INTEGER NOT NULL DEFAULT 0;

ALTER TABLE node_metric_samples ADD COLUMN swap_used_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_metric_samples ADD COLUMN swap_total_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_metric_samples ADD COLUMN disk_read_bytes_per_second INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_metric_samples ADD COLUMN disk_write_bytes_per_second INTEGER NOT NULL DEFAULT 0;

CREATE INDEX node_metric_samples_bucket_idx ON node_metric_samples(bucket_at);
CREATE INDEX node_operations_history_idx
    ON node_operations(created_at DESC, node_id, type, status);
