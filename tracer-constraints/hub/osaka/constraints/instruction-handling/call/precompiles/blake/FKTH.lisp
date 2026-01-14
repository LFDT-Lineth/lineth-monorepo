(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;    X.Y.22.5  The FAILURE_KNOWN_TO_HUB case    ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---BLAKE2f---FAILURE_KNOWN_TO_HUB)    (*    PEEK_AT_SCENARIO
                                                                            scenario/PRC_BLAKE2f
                                                                            scenario/PRC_FAILURE_KNOWN_TO_HUB))

(defconstraint    precompile-processing---BLAKE2f---final-context-row-for-failure-known-to-HUB
                  (:guard    (precompile-processing---BLAKE2f---FAILURE_KNOWN_TO_HUB))
                  (nonexecution-provides-empty-return-data    precompile-processing---BLAKE2f---context-row-offset---squashing-caller-return-data-for-FKTH))
