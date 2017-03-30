ALTER TABLE scheduling_queue
  ADD CONSTRAINT scheduling_queue_uniq_node_path
  UNIQUE (node_path);
