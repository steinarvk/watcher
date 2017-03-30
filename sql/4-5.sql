ALTER TABLE work_leases
  DROP CONSTRAINT work_leases_pkey;

CREATE INDEX work_leases_idx_lease_key
  ON work_leases (lease_key);

ALTER TABLE work_leases ADD COLUMN
  lease_id BIGSERIAL PRIMARY KEY;
