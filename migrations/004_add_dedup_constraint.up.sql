ALTER TABLE telemetry_events
    ADD CONSTRAINT uq_telemetry_device_time UNIQUE (device_id, recorded_at);
