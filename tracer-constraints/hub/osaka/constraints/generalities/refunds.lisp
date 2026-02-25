(module hub)

;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;   4.5 Refunds   ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   4.5.1 Introduction              ;;
;;   4.5.2 Gas column generalities   ;;
;;   4.5.3 Constraints               ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (same-refund-counter   relof)   (eq!  (shift  REFUND_COUNTER_NEW  relof)
                                               (shift  REFUND_COUNTER      relof)
                                               ))

(defun   (increment-refund-counter-by   relof   delta)   (eq!  (shift      REFUND_COUNTER_NEW   relof)
                                                               (+ (shift   REFUND_COUNTER       relof)   delta)
                                                               ))

(defun   (link-current-refund-to-previously-updated-refund)   (eq!  REFUND_COUNTER
                                                                    (shift  REFUND_COUNTER_NEW  -1)
                                                                    ))

;;----------------------------;;
;;   High level constraints   ;;
;;----------------------------;;


(defconstraint    generalities---refunds---only-USER-transactions-may-incur-refunds ()
                  (if-zero    USER
                              (begin
                                ( vanishes!   REFUND_COUNTER     )
                                ( vanishes!   REFUND_COUNTER_NEW )
                                )))

(defconstraint    generalities---refunds---HUB-stamp-constancies ()
                  (begin
                    ( hub-stamp-constancy   REFUND_COUNTER     )
                    ( hub-stamp-constancy   REFUND_COUNTER_NEW )
                    ))


;;--------------------------------------------;;
;;   Initialization and linking constraints   ;;
;;--------------------------------------------;;


(defconstraint    generalities---refunds---initialization-at-transaction-start ()
                  (if-not-zero   (-   TOTL_TXN_NUMBER   (prev   TOTL_TXN_NUMBER))
                                 ( vanishes! REFUND_COUNTER )
                                 ))

(defconstraint    generalities---refunds---linking-during-transaction-execution ()
                  (if-not-zero   (-   TOTL_TXN_NUMBER   (+  1  (prev   TOTL_TXN_NUMBER)))
                                 (if-not-zero   (-  HUB_STAMP   (prev  HUB_STAMP))
                                                (link-current-refund-to-previously-updated-refund)
                                                )))


;;-------------------------------------------------------;;
;;   Transaction processing phase specific constraints   ;;
;;-------------------------------------------------------;;


(defconstraint    generalities---refunds---refunds-remain-constant-along-certain-transaction-processing-phases ()
                  (if-not-zero   (+   TX_SKIP
                                      TX_WARM
                                      TX_INIT
                                      TX_FINL)
                                 (same-refund-counter   0 )
                                 ))

(defconstraint    generalities---refunds---authorization-phase-induced-refunds ()
                  (if-not-zero   TX_AUTH
                                 (if-not-zero  PEEK_AT_AUTHORIZATION
                                               (increment-refund-counter-by   0
                                                                              (auth-tuple-refund))
                                               )))

(defun    (auth-tuple-refund)    (*   auth/AUTHORIZATION_TUPLE_IS_VALID
                                      (shift account/EXISTS  1)
                                      (-   GAS_CONST_PER_EMPTY_ACCOUNT   GAS_CONST_PER_AUTH_BASE_COST )))

(defconstraint    generalities---refunds---reverting-frames-dont-accrue-refunds (:guard  CN_WILL_REV)
                  (same-refund-counter  0))

(defconstraint    generalities---refunds---non-SSTORE-opcodes-dont-accrue-refunds (:perspective stack)
                  (if-not-zero    (opcode-isnt-SSTORE)
                                  (same-refund-counter  0)))

(defun    (opcode-is-SSTORE)     (force-bin  (* stack/STO_FLAG  [stack/DEC_FLAG 2]))) ;; ""
(defun    (opcode-isnt-SSTORE)   (force-bin  (-  1  (opcode-is-SSTORE))))

;; the actual REFUND mechanics for SSTORE will be explained in the storage instruction family section
;; Note: SELFDESTRUCT doesn't accrue refunds anymore since ... Cancun or so.

