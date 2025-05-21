(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                   ;;
;;    X.Y.Z.3 firstRowOfCalleeContext constraints    ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (first-row-of-callee-context    relative_row_offset)
  (begin
    (eq!          (shift    CALLER_CONTEXT_NUMBER    (+    relative_row_offset    1))    (shift    CONTEXT_NUMBER             relative_row_offset))
    (eq!          (shift    CONTEXT_NUMBER           (+    relative_row_offset    1))    (shift    context/CONTEXT_NUMBER     relative_row_offset))
    (eq!          (shift    CODE_FRAGMENT_INDEX      (+    relative_row_offset    1))    (call-instruction---callee-code-fragment-index))
    (vanishes!    (shift    PROGRAM_COUNTER          (+    relative_row_offset    1)))
    (eq!          (shift    GAS_EXPECTED             (+    relative_row_offset    1))    (+    (call-instruction---STP-gas-paid-out-of-pocket)    (call-instruction---STP-call-stipend)))
    (eq!          (shift    GAS_EXPECTED             (+    relative_row_offset    1))    (shift    GAS_ACTUAL    (+    relative_row_offset    1)))
    ))
