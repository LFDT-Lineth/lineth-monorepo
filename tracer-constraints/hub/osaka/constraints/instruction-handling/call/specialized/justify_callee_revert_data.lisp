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
;;    X.Y.Z.1 justifyCalleeRevertData constraints    ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (justify-callee-revert-data    relative_row_offset)
  (begin
    (eq!    (call-instruction---callee-self-reverts)    (shift    CONTEXT_SELF_REVERTS    (+    relative_row_offset    1)))
    (eq!    (call-instruction---callee-revert-stamp)    (shift    CONTEXT_REVERT_STAMP    (+    relative_row_offset    1)))
    ))
