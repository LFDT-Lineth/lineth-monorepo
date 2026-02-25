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


(defun    (justify-callee-revert-data    relof)
  (begin
    (eq!    (call-instruction---callee---self-reverts)    (shift    CONTEXT_SELF_REVERTS    (+    relof    1)))
    (eq!    (call-instruction---callee---revert-stamp)    (shift    CONTEXT_REVERT_STAMP    (+    relof    1)))
    ))
