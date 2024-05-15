(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   3.5 Constraints for peeking flags   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint peeking-flags-are-binary ()
               (begin
                 (is-binary PEEK_AT_ACCOUNT)
                 (is-binary PEEK_AT_CONTEXT)
                 (is-binary PEEK_AT_MISCELLANEOUS)
                 (is-binary PEEK_AT_SCENARIO)
                 (is-binary PEEK_AT_STACK)
                 (is-binary PEEK_AT_STORAGE)
                 (is-binary PEEK_AT_TRANSACTION)))

(defun (peeking_flag_sum)
  (+ PEEK_AT_ACCOUNT
     PEEK_AT_CONTEXT
     PEEK_AT_MISCELLANEOUS
     PEEK_AT_SCENARIO
     PEEK_AT_STACK
     PEEK_AT_STORAGE
     PEEK_AT_TRANSACTION))

(defconstraint peeking-sum-and-transaction-phase-sums-coincide ()
               (eq! (transaction_phase_sum) (peeking_flag_sum)))
