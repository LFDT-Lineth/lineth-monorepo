(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;    X.Y.Z.4 MODEXP common processing    ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---MODEXP---standard-precondition)    (*    PEEK_AT_SCENARIO    scenario/PRC_MODEXP))

(defconstraint    precompile-processing---MODEXP---excluding-execution-scenarios
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (vanishes!    scenario/PRC_FAILURE_KNOWN_TO_HUB))

