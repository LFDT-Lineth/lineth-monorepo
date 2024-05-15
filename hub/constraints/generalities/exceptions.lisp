(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;   4.1 Exception flags and EXCEPTION_AHOY   ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (exception_flag_sum)
                (+  stack/OPCX
                    stack/SUX
                    stack/SOX
                    stack/OOGX
                    stack/MXPX
                    stack/RDCX
                    stack/JUMPX
                    stack/STATICX
                    stack/SSTOREX
                    stack/ICPX
                    stack/MAXCSX))

(defun (weighted_exception_flag_sum)
                (+  (* 1   stack/OPCX)
                    (* 2   stack/SUX)
                    (* 4   stack/SOX)
                    (* 8   stack/OOGX)
                    (* 16  stack/RDCX)
                    (* 32  stack/JUMPX)
                    (* 64  stack/STATICX)
                    (* 128 stack/SSTOREX)
                    (* 256 stack/ICPX)
                    (* 512 stack/MAXCSX)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   4.1.1 Binarity and constancy conditions   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint exception-flags-are-binary (:perspective stack)
               (begin
                 (is-binary  SUX     )
                 (is-binary  SOX     )
                 (is-binary  OOGX    )
                 (is-binary  MXPX    )
                 (is-binary  OPCX    ) 
                 (is-binary  RDCX    )
                 (is-binary  JUMPX   )
                 (is-binary  STATICX )
                 (is-binary  SSTOREX )
                 (is-binary  ICPX    )
                 (is-binary  MAXCSX  )))


(defconstraint exception-flags-are-exclusive (:perspective stack)
               (is-binary (exception_flag_sum)))


(defconstraint exception-flags-are-stack-constant (:perspective stack)
               (stack-row-constancy  (weighted_exception_flag_sum)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;   4.1.2 Automatic vanishing constraints   ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint automatic-exception-flag-vanishing (:perspective stack)
               (begin
                 (eq!            INVALID_FLAG                    OPCX)
                 (if-zero        MXP_FLAG                        (vanishes! MXPX))
                 (if-zero        JUMP_FLAG                       (vanishes! JUMPX))
                 (if-zero        STATIC_FLAG                     (vanishes! STATICX))
                 (if-not-zero    (-  INSTRUCTION EVM_INST_RETURNDATACOPY) (vanishes! RDCX))
                 (if-not-zero    (-  INSTRUCTION EVM_INST_SSTORE)         (vanishes! SSTOREX))
                 (if-not-zero    (-  INSTRUCTION EVM_INST_RETURN)         (vanishes! (+ ICPX MAXCSX)))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;   4.1.3 The XAHOY flag   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; we deal with those constraints in context.lisp along side CMC

;; (defconstraint exception_ahoy ()
;;                (begin
;;                  (is-binary                             XAHOY)
;;                  (hub-stamp-constancy                   XAHOY)
;;                  (if-zero TX_EXEC            (vanishes! XAHOY))
;;                  (if-not-zero PEEK_AT_STACK
;;                               (eq! (exception_flag_sum) XAHOY))))
