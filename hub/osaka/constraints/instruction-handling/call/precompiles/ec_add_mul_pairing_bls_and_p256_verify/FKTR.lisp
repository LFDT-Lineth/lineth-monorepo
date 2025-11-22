(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                     ;;
;;    X.Y.Z ECADD, ECMUL, ECPAIRING and BLS precompiles constraints    ;;
;;                                                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;
;; Shorthands ;;
;;;;;;;;;;;;;;;;

(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---FKTR-case)    (*    PEEK_AT_SCENARIO
                                                                                       (+    scenario/PRC_ECADD
                                                                                             scenario/PRC_ECMUL
                                                                                             scenario/PRC_ECPAIRING
                                                                                             (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                                                                                             )
                                                                                       scenario/PRC_FAILURE_KNOWN_TO_RAM))

(defconstraint    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---FKTR-requires-extracting-non-empty-call-data
                  (:guard    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---FKTR-case))
                  (eq!    (precompile-processing---common---OOB-extract-call-data)
                          1))

