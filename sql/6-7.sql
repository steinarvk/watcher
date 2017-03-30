ALTER TABLE work_leases
  ADD CONSTRAINT work_leases_uniq_lease_key
  UNIQUE (lease_key);
