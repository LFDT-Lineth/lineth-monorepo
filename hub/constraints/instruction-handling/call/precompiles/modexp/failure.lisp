(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;    X.Y.Z.4 MODEXP failure case    ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---MODEXP---failure)    (*    PEEK_AT_SCENARIO
                                                              scenario/PRC_MODEXP
                                                              (scenario-shorthand---PRC---failure)))

(defconstraint    precompile-processing---MODEXP---failure---provide-empty-return-data-to-caller    (:guard    (precompile-processing---MODEXP---failure))
                  (nonexecution-provides-empty-return-data    precompile-processing---MODEXP---context-row-offset---FKTR))
