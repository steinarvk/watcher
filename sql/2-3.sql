ALTER TABLE program_executions ADD COLUMN
  executor_host TEXT NOT NULL;

ALTER TABLE program_executions ADD COLUMN
  executor_pid INT NOT NULL;
