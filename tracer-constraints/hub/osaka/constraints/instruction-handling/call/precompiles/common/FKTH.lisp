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
;;    X.Y.Z.4 The  PRC/FAILURE_KNOWN_TO_HUB case    ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---common---FKTH-precondition)    (*    PEEK_AT_SCENARIO
                                                                        (scenario-shorthand---PRC---common-address-bit-sum)
                                                                        scenario/PRC_FAILURE_KNOWN_TO_HUB))

(defconstraint    precompile-processing---common---setting-the-context-row-FKTH-case    (:guard (precompile-processing---common---FKTH-precondition))
                  (nonexecution-provides-empty-return-data    precompile-processing---common---context-row-FKTH---row-offset))
