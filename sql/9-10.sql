ALTER TABLE program_executions ADD COLUMN
  root_execution_id BIGINT NULL
    REFERENCES program_executions (execution_id)
    ON DELETE CASCADE;
