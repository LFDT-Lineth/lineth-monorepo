(defun (unexceptional-stack-row)
  (force-bin (* hub.PEEK_AT_STACK
                (- 1 hub.XAHOY))))

(defun (unexceptional-stack-row-logical)
  (and! (!= 0 hub.PEEK_AT_STACK)
        (== 0 hub.XAHOY)))
