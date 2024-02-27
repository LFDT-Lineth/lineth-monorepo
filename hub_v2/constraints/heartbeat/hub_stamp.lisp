(module hub_v2)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   3.6 Constraints for the hub stamp   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint hub-stamp-initial-vanishing (:domain {0})
  (vanishes! HUB_STAMP))

(defconstraint hub-stamp-0-1-increments ()
  (any! (will-inc! HUB_STAMP 0) (will-inc! HUB_STAMP 1)))

(defconstraint hub-stamp-jumps-at-transaction-phase-boundaries ()
  (let ((tx_phase_transition_bit (+ (* (- 1 TX_SKIP) (next TX_SKIP))
                                    (* (- 1 TX_WARM) (next TX_WARM))
                                    (* (- 1 TX_INIT) (next TX_INIT))
                                    (* (- 1 TX_EXEC) (next TX_EXEC))
                                    (* (- 1 TX_FINL) (next TX_FINL))
                                    TX_WARM)))
       (if-not-zero tx_phase_transition_bit
                    (will-inc! HUB_STAMP 1))))

(defconstraint hub-stamp-remains-constant-during-skipping-phase ()
  (if-not-zero (+ TX_SKIP TX_INIT TX_FINL)
               (will-eq! HUB_STAMP (+ HUB_STAMP PEEK_AT_TRANSACTION))))

(defconstraint hub-stamp-jumps-at-transaction-boundaries ()
  ;; corset doesn't like empty constraints
  (debug (if-not-zero (will-remain-constant! ABS_TX_NUM)
                      (will-inc! HUB_STAMP 1))))

(defconstraint hub-stamp-increments-during-execution-phase ()
  (if-not-zero TX_EXEC
               (begin (if-not-zero (remained-constant! HUB_STAMP)
                                   (begin (vanishes! COUNTER_TLI)
                                          (vanishes! COUNTER_NSR)))
                      (if-zero COUNTER_NSR
                               (eq! PEEK_AT_STACK 1)
                               (vanishes! PEEK_AT_STACK))
                      (if-eq-else COUNTER_TLI TLI
                                  ;; CT_TLI = #TLI
                                  (if-eq-else COUNTER_NSR NSR
                                              ;; CT_NSR = #NSR
                                              (will-inc! HUB_STAMP 1)
                                              ;; CT_NSR ≠ #NSR
                                              (begin (will-remain-constant! HUB_STAMP)
                                                     (will-remain-constant! COUNTER_TLI)
                                                     (will-inc! COUNTER_NSR 1)))
                                  ;; CT_TLI ≠ #TLI
                                  (begin (will-remain-constant! HUB_STAMP)
                                         (will-inc! COUNTER_TLI 1)
                                         (vanishes! (next COUNTER_NSR)))))))


