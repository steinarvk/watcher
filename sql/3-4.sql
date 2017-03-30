CREATE INDEX program_executions_idx_node_path
  ON program_executions (node_path);

CREATE INDEX program_executions_idx_parent_execution_id
  ON program_executions (parent_execution_id);
