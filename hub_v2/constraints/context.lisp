(module hub_v2)

(defun (provide_return_data recipient provider is_prec offset size)
 (begin
  (eq! CONTEXT_NUMBER recipient)
  (eq! UPDATE 1)
  (eq! RETURNER_CONTEXT_NUMBER provider)
  (eq! RETURNER_IS_PRECOMPILE is_prec)
  (eq! RETURN_DATA_OFFSET offset)
  (eq! RETURN_DATA_SIZE size)))

(defun (execution_provides_empty_return_data)
 (provide_return_data CALLER_CONTEXT_NUMBER CONTEXT_NUMBER 0 0 0))

(defun (non_context_provides_empty_return_date)
 (provide_return_data CONTEXT_NUMBER (+ HUB_STAMP 1) 0 0 0))

(defun (read_context_data context_number)
 (begin
  (eq! CONTEXT_NUMBER context_number)
  (vanishes! UPDATE)))