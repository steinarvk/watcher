ALTER TABLE program_executions
  ADD CONSTRAINT
    program_executions_uniq_node_and_start
    UNIQUE (node_path, started_utcmillis);
