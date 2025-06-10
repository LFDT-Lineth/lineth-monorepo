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
;;    X.Y.22.5  The FAILURE_KNOWN_TO_RAM case    ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---BLAKE2f---FAILURE_KNOWN_TO_RAM)    (*    PEEK_AT_SCENARIO
                                                                            scenario/PRC_BLAKE2f
                                                                            scenario/PRC_FAILURE_KNOWN_TO_RAM))

(defconstraint    precompile-processing---BLAKE2f---final-context-row-for-failure-known-to-RAM
                  (:guard    (precompile-processing---BLAKE2f---FAILURE_KNOWN_TO_RAM))
                  (nonexecution-provides-empty-return-data    precompile-processing---BLAKE2f---context-row-offset---squashing-caller-return-data-for-FKTR))
