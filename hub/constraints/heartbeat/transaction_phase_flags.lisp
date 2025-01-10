(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   3.4 Transaction phase flags   ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; (defconstraint transaction-phase-binarities ()
;;                (begin
;;                  (is-binary TX_SKIP)
;;                  (is-binary TX_WARM)
;;                  (is-binary TX_INIT)
;;                  (is-binary TX_EXEC)
;;                  (is-binary TX_FINL)))

(defun (transaction_phase_sum)
  (+ TX_SKIP
     TX_WARM
     TX_INIT
     TX_EXEC
     TX_FINL))

(defconstraint transaction-phase-sum ()
               (begin
                 (if-zero ABS_TX_NUM
                          (eq! (transaction_phase_sum) 0)
                          (eq! (transaction_phase_sum) 1))))

(defconstraint first-phase-of-new-transaction ()
               (if-not-zero (remained-constant! ABS_TX_NUM)
                            (eq! (+ TX_SKIP TX_WARM TX_INIT) 1)))

(defconstraint abs-tx-num-increments ()
               (if-not-zero ABS_TX_NUM
                            (eq!
                              (next ABS_TX_NUM)
                              (+ ABS_TX_NUM (* (+ TX_FINL TX_SKIP) PEEK_AT_TRANSACTION)))))

(defconstraint remaining-in-tx-skip-phase ()
               (if-not-zero TX_SKIP
                            (if-zero PEEK_AT_TRANSACTION
                                     (eq! (next TX_SKIP) 1))))

(defconstraint permissible-phase-transitions ()
               (begin
                 (if-not-zero TX_WARM
                              (eq! (next (+ TX_WARM TX_INIT)) 1))
                 (if-not-zero TX_INIT
                              (if-zero PEEK_AT_CONTEXT
                                       (eq! (next TX_INIT) 1)
                                       (eq! (next TX_EXEC) 1)))
                 (if-not-zero TX_EXEC
                              (eq! (next (+ TX_EXEC TX_FINL)) 1))
                 (if-not-zero TX_FINL
                              (eq! (next TX_FINL)
                                   (- 1 PEEK_AT_TRANSACTION)))))
