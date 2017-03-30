CREATE TABLE work_leases (
  lease_key TEXT PRIMARY KEY,
  leased_until_utcmillis BIGINT NOT NULL
);

CREATE TABLE program_executions (
  execution_id BIGSERIAL PRIMARY KEY,
  parent_execution_id BIGINT NULL REFERENCES program_executions (execution_id),
  node_path TEXT NOT NULL,
  started_utcmillis BIGINT NOT NULL,
  stopped_utcmillis BIGINT NOT NULL,
  success BOOL NOT NULL,
  stdout TEXT NOT NULL,
  stderr TEXT NOT NULL
);

CREATE TABLE scheduling_queue (
  schedule_id BIGSERIAL PRIMARY KEY,
  node_path TEXT NOT NULL,
  target_time_utcmillis BIGINT NOT NULL
);
