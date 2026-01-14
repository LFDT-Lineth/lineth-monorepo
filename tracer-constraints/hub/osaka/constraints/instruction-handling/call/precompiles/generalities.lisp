(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    X.Y.Z.4 Generalities    ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---standard-hypothesis)    (*    PEEK_AT_SCENARIO   (scenario-shorthand---PRC---sum)))


(defconstraint    precompile-processing---admissible-failure-scenarios
                  (:guard (precompile-processing---standard-hypothesis))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero    (scenario-shorthand---PRC---may-only-fail-in-the-HUB)    (vanishes!    scenario/PRC_FAILURE_KNOWN_TO_RAM))
                    (if-not-zero    (scenario-shorthand---PRC---may-only-fail-in-the-RAM)    (vanishes!    scenario/PRC_FAILURE_KNOWN_TO_HUB))
                    ))

(defconstraint    precompile-processing---setting-GAS_NEXT
                  (:guard (precompile-processing---standard-hypothesis))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    GAS_NEXT    (+    (precompile-processing---dup-caller-gas)
                                              (precompile-processing---prd-return-gas)))
                    (if-not-zero    (scenario-shorthand---PRC---failure)
                                    (vanishes!    (precompile-processing---prd-return-gas)))
                    ))

(defconstraint    precompile-processing---setting-NSR
                  (:guard (precompile-processing---standard-hypothesis))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    NON_STACK_ROWS
                          (+    (precompile-processing---1st-half-NSR)
                                (precompile-processing---2nd-half-NSR)
                                )))

(defconstraint    precompile-processing---setting-the-peeking-flags
                  (:guard (precompile-processing---standard-hypothesis))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (precompile-processing---2nd-half-flag-sum)
                          (precompile-processing---2nd-half-NSR)
                          ))
