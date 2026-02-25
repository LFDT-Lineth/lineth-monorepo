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


(defun    (first-row-of-child-context    relof)
  (begin
    (eq!          (shift    CALLER_CONTEXT_NUMBER    (+    relof    1))    (shift    CONTEXT_NUMBER             relof))
    (eq!          (shift    CONTEXT_NUMBER           (+    relof    1))    (shift    context/CONTEXT_NUMBER     relof))
    (eq!          (shift    CODE_FRAGMENT_INDEX      (+    relof    1))    (call-instruction---delegate-or-callee---cfi))
    (vanishes!    (shift    PROGRAM_COUNTER          (+    relof    1)))
    (eq!          (shift    GAS_EXPECTED             (+    relof    1))    (+    (call-instruction---STP-gas-paid-out-of-pocket)    (call-instruction---STP-call-stipend)))
    (eq!          (shift    GAS_EXPECTED             (+    relof    1))    (shift    GAS_ACTUAL    (+    relof    1)))
    ))
