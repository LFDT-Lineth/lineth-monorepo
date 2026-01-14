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

(defconstraint    generalities---gas---refund-column-constancies ()
                  (begin
                    (hub-stamp-constancy REFUND_COUNTER)
                    (hub-stamp-constancy REFUND_COUNTER_NEW)))

(defconstraint    generalities---gas---refunds-vanish-outside-of-execution-rows ()
                  (if-zero    TX_EXEC
                              (begin
                                (vanishes! REFUND_COUNTER)
                                (vanishes! REFUND_COUNTER_NEW))))

(defconstraint    generalities---gas---refunds-transition-constraints ()
                  (if-not-zero    TX_EXEC
                                  (if-not    (remained-constant! HUB_STAMP)
                                             (eq! REFUND_COUNTER (prev REFUND_COUNTER_NEW)))))

(defconstraint    generalities---gas---discard-refunds-if-context-will-revert ()
                  (if-not-zero    CN_WILL_REV
                                  (eq! REFUND_COUNTER_NEW REFUND_COUNTER)))

(defun    (bit-identifying-SSTORE)   (* stack/STO_FLAG  [stack/DEC_FLAG 2]))     ;; ""

(defconstraint    generalities---gas---only-SSTORE-may-grant-refunds (:perspective stack)
                  (if-zero    (force-bin (bit-identifying-SSTORE))
                              (eq! REFUND_COUNTER_NEW REFUND_COUNTER)))

;; the actual REFUND mechanics for SSTORE will be explained in the storage instruction family section
