(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  1.3 Module call macros  ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (wcp-call-to-LT    offset
                          a_hi
                          a_lo
                          b_hi
                          b_lo)
  (begin (eq! (shift WCP_FLAG offset) 1)
         (eq! (shift EXO_INST offset) EVM_INST_LT)
         (eq! (shift ARG_1_HI offset) a_hi)
         (eq! (shift ARG_1_LO offset) a_lo)
         (eq! (shift ARG_2_HI offset) b_hi)
         (eq! (shift ARG_2_LO offset) b_lo)
         (eq! (shift RES      offset) 1)))

(defun (wcp-call-to-GT    offset
                          a_hi
                          a_lo
                          b_hi
                          b_lo)
  (begin (eq! (shift WCP_FLAG offset) 1)
         (eq! (shift EXO_INST offset) EVM_INST_GT)
         (eq! (shift ARG_1_HI offset) a_hi)
         (eq! (shift ARG_1_LO offset) a_lo)
         (eq! (shift ARG_2_HI offset) b_hi)
         (eq! (shift ARG_2_LO offset) b_lo)
         (eq! (shift RES      offset) 1)))

(defun (wcp-call-to-LEQ   offset
                          a_hi
                          a_lo
                          b_hi
                          b_lo)
  (begin
    (eq! (shift WCP_FLAG offset) 1)
    (eq! (shift EXO_INST offset) WCP_INST_LEQ)
    (eq! (shift ARG_1_HI offset) a_hi)
    (eq! (shift ARG_1_LO offset) a_lo)
    (eq! (shift ARG_2_HI offset) b_hi)
    (eq! (shift ARG_2_LO offset) b_lo)
    (eq! (shift RES      offset) 1)))

(defun (wcp-call-to-GEQ   offset
                          a_hi
                          a_lo
                          b_hi
                          b_lo)
  (begin (eq! (shift WCP_FLAG offset) 1)
         (eq! (shift EXO_INST offset) WCP_INST_GEQ)
         (eq! (shift ARG_1_HI offset) a_hi)
         (eq! (shift ARG_1_LO offset) a_lo)
         (eq! (shift ARG_2_HI offset) b_hi)
         (eq! (shift ARG_2_LO offset) b_lo)
         (eq! (shift RES      offset) 1)))

(defun (wcp-call-to-ISZERO    offset
                              a_hi
                              a_lo)
  (begin (eq! (shift WCP_FLAG offset) 1)
         (eq! (shift EXO_INST offset) EVM_INST_ISZERO)
         (eq! (shift ARG_1_HI offset) a_hi)
         (eq! (shift ARG_1_LO offset) a_lo)
         (eq! (shift ARG_2_HI offset) 0)
         (eq! (shift ARG_2_LO offset) 0)))

(defun (euc-call    offset
                    a
                    b)
  (begin (eq! (shift EUC_FLAG offset) 1)
         (eq! (shift ARG_1_LO offset) a)
         (eq! (shift ARG_2_LO offset) b)))
