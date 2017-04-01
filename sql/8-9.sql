ALTER TABLE program_executions
  DROP CONSTRAINT program_executions_parent_execution_id_fkey;

ALTER TABLE program_executions
  ADD CONSTRAINT program_executions_parent_execution_id_fkey
  FOREIGN KEY (parent_execution_id)
  REFERENCES program_executions (execution_id)
  ON DELETE CASCADE;
