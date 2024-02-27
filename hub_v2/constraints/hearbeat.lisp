(module hub_v2)

;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;
;;;;               ;;;;
;;;;  3 Heartbeat  ;;;;
;;;;               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  3.2 ABS_TX# and BATCH#  ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint  first_row   (:domain {0})
                (begin
                    (vanishes! ABSOLUTE_TRANSACTION_NUMBER)
                    (debug  (vanishes! BATCH_NUMBER))))

(defconstraint  update  ()
                (begin
                    (vanishes!   (*  (will-inc! ABSOLUTE_TRANSACTION_NUMBER 0) (will-inc! ABSOLUTE_TRANSACTION_NUMBER 1)))
                    (debug  (vanishes!   (*  (will-inc! BATCH_NUMBER 0)    (will-inc! BATCH_NUMBER 1))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;  3.3 Transaction phase flags  ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint binary_phase_flags ()
                (begin
                    (is-binary  TX_SKIP)
                    (is-binary  TX_WARM)
                    (is-binary  TX_INIT)
                    (is-binary  TX_EXEC)
                    (is-binary  TX_FINL)))

(defun  (sum_of_phase_flags)    (+ TX_SKIP TX_WARM TX_INIT TX_EXEC TX_FINL))
(defun  (first_flag_of_tx)      (+ TX_SKIP TX_WARM TX_INIT))

(defconstraint one-phase ()
                (if-zero    ABSOLUTE_TRANSACTION_NUMBER
                    (eq!  (sum_of_phase_flags)  0)
                    (eq!  (sum_of_phase_flags)  1)))

(defconstraint first_phase_of_tx    ()
                (if-not-zero    (remained-constant!    ABSOLUTE_TRANSACTION_NUMBER)
                    (eq!  (first_flag_of_tx)  1)))

(defconstraint  abs_tx_num_update   ()
                (if-not-zero    ABSOLUTE_TRANSACTION_NUMBER
                    (will-eq!    ABSOLUTE_TRANSACTION_NUMBER
                                (+  ABSOLUTE_TRANSACTION_NUMBER
                                    (*  (+  TX_FINL  TX_SKIP)
                                        PEEK_AT_TRANSACTION)))))

;; Transactions whose processing requires no evm execution.
(defconstraint transactions-without-evm-execution ()
                (if-not-zero    TX_SKIP
                    (if-zero PEEK_AT_TRANSACTION
                        (eq!  (next TX_SKIP) 1))))

;; Transactions whose processing does require evm execution.
(defconstraint  transactions-with-evm-execution ()
                (begin
                    (if-not-zero TX_WARM    (= 1    (next  (+  TX_WARM TX_INIT))))
                    (if-not-zero TX_INIT    (if-zero    PEEK_AT_TRANSACTION
                                                (eq!  (next    TX_INIT)   1)
                                                (eq!  (next    TX_EXEC)   1)))
                    (if-not-zero TX_EXEC    (eq! 1    (next  (+  TX_EXEC TX_FINL))))
                    (if-not-zero TX_FINL    (if-zero    PEEK_AT_TRANSACTION
                                                (eq!  1   (next TX_FINL))))))


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;  3.4 Peeking flags  ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint peeking-flags-are-binary ()
                (begin
                    (is-binary  PEEK_AT_STACK)
                    (is-binary  PEEK_AT_CONTEXT)
                    (is-binary  PEEK_AT_ACCOUNT)
                    (is-binary  PEEK_AT_STORAGE)
                    (is-binary  PEEK_AT_TRANSACTION)))

(defun (sum_of_peeking_flags)
                (+  PEEK_AT_STACK
                    PEEK_AT_CONTEXT
                    PEEK_AT_ACCOUNT
                    PEEK_AT_STORAGE
                    PEEK_AT_TRANSACTION))

(defconstraint peeking-flags-activation ()
                (if-zero ABSOLUTE_TRANSACTION_NUMBER
                    (vanishes!   (sum_of_peeking_flags))
                    (eq! 1        (sum_of_peeking_flags))))


;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;  3.4 Hub stamp  ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;

;; (jumps-to-one X) should be applied to binary columns only
;; = 1 at row i iff X[i] = 0 and X[i + 1] = 1
(defun (jumps-to-one X)  (*  (next X) (- 1 X)))

;; (stays-at-one X) should only be applied to binary colunms
;; = 1 at row i iff X[i] = X[i + 1] = 1
(defun (stays-at-one X)  (*  X   (next X)))

;; (defconstraint hub_stamp_constraints    ()
;;                 (begin
;;                     (if-zero ABSOLUTE_TRANSACTION_NUMBER    (vanishes! HUB_STAMP))           ;; initialization
;;                     (vanishes!   (*  (will-remain-constant!  HUB_STAMP)    (will-inc! HUB_STAMP 1)))     ;; stamp incrememts
;;                     (if-not-zero    (+  (jumps-to-one TX_SKIP)
;;                                         (jumps-to-one TX_WARM)
;;                                         (jumps-to-one TX_INIT)
;;                                         (jumps-to-one TX_EXEC)
;;                                         (jumps-to-one TX_FINL))
;;                         (will-inc!    HUB_STAMP 1))
;;                     (if-not-zero (will-remain-constant!  ABSOLUTE_TRANSACTION_NUMBER)    (will-inc! HUB_STAMP 1))
;;                     (if-not-zero TX_SKIP (if-zero PEEK_AT_TRANSACTION (will-remain-constant!  HUB_STAMP)))
;;                     (if-not-zero    (+  (stays-at-one   TX_WARM)
;;                                         (stays-at-one   TX_INIT)
;;                                         (stays-at-one   TX_FINL))
;;                                     (will-remain-constant!   HUB_STAMP))
;;                     (if-not-zero TX_EXEC
;;                         (if-not-zero    (remained-constant!    HUB_STAMP)
;;                                             (begin
;;                                                 (vanishes!   COUNTER_TLI)
;;                                                 (vanishes!   COUNTER_NSR)))
;;                         (if-zero    CT_NSR
;;                             (= 1        PEEK_AT_STACK)
;;                             (vanishes!   PEEK_AT_STACK))
;;                         (if! (neq!  CT_TLI  TLI)
;;                             (begin
;;                                 (will-remain-constant!    HUB_STAMP)
;;                                 (will-inc!    CT_TLI 1)
;;                                 (vanishes!   (next   CT_NSR))))
;;                         (if-eq      CT_TLI  TLI
;;                             (if-not-zero    (-  CT_NSR NSR)
;;                                 (begin
;;                                     (will-remain-constant!    HUB_STAMP)
;;                                     (will-remain-constant!    COUNTER_TLI)
;;                                     (will-inc!                COUNTER_NSR 1))
;;                                 (will-inc!    HUB_STAMP 1))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;  3.6 MEMORY_EXPANSION_STAMP (generalities)  ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; (defconstraint mxp_stamp_starts_at_zero (:domain {0}) (vanishes! MEMORY_EXPANSION_STAMP))
;; (defconstraint mxp_stamp_generalities ()
;;                 (begin
;;                     (vanishes!   (*  (will-remain-constant!    MEMORY_EXPANSION_STAMP)
;;                                     (will-inc!    MEMORY_EXPANSION_STAMP   1)))
;;                     (if-not-zero    (remained-constant!    HUB_STAMP)
;;                         (if-zero PEEK_AT_STACK
;;                             (remained-constant!  MEMORY_EXPANSION_STAMP)
;;                             (if-zero    MXP_FLAG
;;                                 (if-not-zero (force-bool (+ SUX + SOX))
;;                                     (remained-constant!    MEMORY_EXPANSION_STAMP)
;;                                     (vanishes! 0)))))))  ;; see section 12


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.7 Final row constraints  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint  final_row_constraint    (:domain {-1} :guard ABSOLUTE_TRANSACTION_NUMBER)
                    (=  (+  TX_SKIP TX_FINL PEEK_AT_TRANSACTION)    2))
