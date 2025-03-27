(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   4.2 Context numbers and context changes   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;   4.1.3 The XAHOY flag                        ;;
;;   4.2.2 Setting the CONTEXT_MAY_CHANGE flag   ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; (defconstraint generalities---setting-the-CONTEXT_MAY_CHANGE-flag ()
;;                (begin
;;                  (is-binary                             CMC)  ;; column already declared :binary@prove
;;                  (hub-stamp-constancy                   CMC)
;;                  (if-zero TX_EXEC            (vanishes! CMC))
;;                  (if-not-zero PEEK_AT_STACK
;;                               (eq! (exception_flag_sum) XAHOY))))


;; we subsume parts of XAHOY and CMC under as follows:
;; (cmc_and_xahoy_weighted_sum) = CMC + 2 * XAHOY

(defun (cmc_and_xahoy_weighted_sum) (+ CMC (* 2 XAHOY)))
(defun (cmc_sum) (+ XAHOY
                    stack/CALL_FLAG
                    stack/CREATE_FLAG
                    stack/HALT_FLAG))

(defconstraint    generalities---setting-CMC-and-XAHOY---stamp-constancies ()
                  ;; this settles hub-stamp-constancy for CMC and XAHOY simultaneously
                  (hub-stamp-constancy   (cmc_and_xahoy_weighted_sum)))

(defconstraint    generalities---setting-CMC-and-XAHOY---automatic-vanishing ()
                  ;; this forces the vanishing of CMC and XAHOY outside of execution rows
                  (if-zero TX_EXEC
                           (vanishes! (cmc_and_xahoy_weighted_sum))))

(defconstraint    generalities---setting-CMC-and-XAHOY---nontrivial-values ()
                  ;; nontrivial values for CMC and XAHOY
                  (if-not-zero PEEK_AT_STACK
                               (begin (eq! (exception_flag_sum) XAHOY)
                                      (if-zero (cmc_sum)
                                               (eq! CMC 0)
                                               (eq! CMC 1)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;   4.2.3 Context numbers   ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    generalities---context-numbers---constancies ()
                  (begin
                    (hub-stamp-constancy CN)
                    (hub-stamp-constancy CN_NEW)))

(defconstraint    generalities---context-numbers---TX_EXEC-phase---CN-vanishes-precisely-when-TX_EXEC-does ()
                  (if-not-zero   CN
                                 (eq!   TX_EXEC   1)
                                 (eq!   TX_EXEC   0)
                                 ))

(defconstraint    generalities---context-numbers---TX_INIT-phase ()
                  (if-not-zero TX_INIT
                               (eq!  CN_NEW (+ 1 HUB_STAMP))))

(defconstraint    generalities---context-numbers---TX_EXEC-phase---CN_NEW-value-options ()
                  (if-not-zero TX_EXEC
                               (or! (eq! CN_NEW CN)
                                    (eq! CN_NEW CALLER_CN)
                                    (eq! CN_NEW (+ 1 HUB_STAMP)))))

(defconstraint    generalities---context-numbers---TX_EXEC-phase---linking-constraint-for-CN-and-CN_NEW ()
                  (if-not-zero TX_EXEC
                               (if-not-zero (remained-constant! HUB_STAMP)
                                            (eq! CN (prev CN_NEW)))))

(defconstraint    generalities---context-numbers---TX_EXEC-phase---CN-may-only-change-if-CMC ()
                  (if-not-zero TX_EXEC
                               (if-zero CMC (eq! CN_NEW CN))))

(defconstraint    generalities---context-numbers---at-HUB_STAMP-transitions ()
                  (if-not-zero (will-remain-constant! HUB_STAMP)
                               (begin
                                 (if-not-zero CMC   (eq! PEEK_AT_CONTEXT 1))
                                 (if-not-zero XAHOY (execution-provides-empty-return-data 0))
                                 (if-not-zero TX_EXEC
                                              (if-not-zero CN_NEW
                                                           (eq! (next TX_EXEC) 1)
                                                           (eq! (next TX_FINL) 1))))))

(defconstraint    generalities---context-numbers---imposing-CN_NEW-is-CALLER_CN---for-exceptions ()
                  (if-not-zero XAHOY (eq! CN_NEW CALLER_CN)))

(defconstraint    generalities---context-numbers---imposing-CN_NEW-is-CALLER_CN---for-halting-instructions ()
                  (if-not-zero PEEK_AT_STACK
                               (if-not-zero stack/HALT_FLAG
                                            (eq! CN_NEW CALLER_CN))))
