(module hub)

;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;   4.5 Refunds   ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;   4.5.1 Introduction   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   4.5.2 Gas column generalities   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; hubStamp constancies already enforced

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;   4.5.3 Constraints   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint refunds-vanish-outside-of-execution-rows ()
               (if-zero TX_EXEC
                        (begin
                          (vanishes! REFUND_COUNTER    )
                          (vanishes! REFUND_COUNTER_NEW))))

(defconstraint refunds-transition-constraints ()
               (if-not-zero (remained-constant! HUB_STAMP)
                            (if-not-zero TX_EXEC
                                         (eq! REFUND_COUNTER (prev REFUND_COUNTER_NEW)))))

(defconstraint discard-refunds-if-context-will-revert ()
               (if-not-zero CN_WILL_REV
                            (eq! REFUND_COUNTER_NEW REFUND_COUNTER)))

(defun (bit-identifying-SSTORE-and-SELFDESTRUCT-instructions) (+ (* stack/STO_FLAG  [stack/DEC_FLAG 1])
                                                                 (* stack/HALT_FLAG [stack/DEC_FLAG 4])))

(defconstraint only-SSTORE-and-SELFDESTRUCT-can-grant-refunds (:perspective stack)
               (if-zero (force-bool (bit-identifying-SSTORE-and-SELFDESTRUCT-instructions))
                        (eq! REFUND_COUNTER_NEW REFUND_COUNTER)))

;; the actual REFUND mechanics will be explained with SSTORE and SELFDESTRUCT instructions
