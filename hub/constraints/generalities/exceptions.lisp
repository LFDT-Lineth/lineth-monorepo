(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;   4.1 Exception flags and EXCEPTION_AHOY   ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (exception_flag_sum)    (+ stack/SUX
                                     stack/SOX
                                     stack/MXPX
                                     stack/OOGX
                                     stack/RDCX
                                     stack/JUMPX
                                     stack/STATICX
                                     stack/SSTOREX
                                     stack/ICPX
                                     stack/MAXCSX
                                     stack/OPCX
                                     ))

(defun    (weighted_exception_flag_sum)    (+ (*   (^ 2  0)    stack/SUX)
                                              (*   (^ 2  1)    stack/SOX)
                                              (*   (^ 2  2)    stack/MXPX)
                                              (*   (^ 2  3)    stack/OOGX)
                                              (*   (^ 2  4)    stack/RDCX)
                                              (*   (^ 2  5)    stack/JUMPX)
                                              (*   (^ 2  6)    stack/STATICX)
                                              (*   (^ 2  7)    stack/SSTOREX)
                                              (*   (^ 2  8)    stack/ICPX)
                                              (*   (^ 2  9)    stack/MAXCSX)
                                              (*   (^ 2 10)    stack/OPCX)
                                              )) ;; ""


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   4.1.1 Binarity and constancy conditions   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint exception-flags-are-binary (:perspective stack)
               (begin
                 (is-binary  SUX     )
                 (is-binary  SOX     )
                 (is-binary  MXPX    )
                 (is-binary  OOGX    )
                 (is-binary  RDCX    )
                 (is-binary  JUMPX   )
                 (is-binary  STATICX )
                 (is-binary  SSTOREX )
                 (is-binary  ICPX    )
                 (is-binary  MAXCSX  )
                 (is-binary  OPCX    )
                 ))


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
                 (eq!            INVALID_FLAG                                   OPCX)
                 (if-zero        MXP_FLAG                                       (vanishes!    MXPX))
                 (if-zero        JUMP_FLAG                                      (vanishes!    JUMPX))
                 (if-zero        STATIC_FLAG                                    (vanishes!    STATICX))
                 (if-not-zero    (-  INSTRUCTION    EVM_INST_RETURNDATACOPY)    (vanishes!    RDCX))
                 (if-not-zero    (-  INSTRUCTION    EVM_INST_SSTORE)            (vanishes!    SSTOREX))
                 (if-not-zero    (-  INSTRUCTION    EVM_INST_RETURN)            (vanishes!    (+ ICPX MAXCSX)))))


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
