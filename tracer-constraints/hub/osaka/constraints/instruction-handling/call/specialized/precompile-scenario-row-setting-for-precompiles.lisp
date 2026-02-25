(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;    X.Y.Z.6 precompileScenarioRow constraints    ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-scenario-row-setting    relof)
  (begin
    (eq!    (shift    PEEK_AT_SCENARIO                                         relof)    1)
    (eq!    (shift    (scenario-shorthand---PRC---sum)                         relof)    1)
    (eq!    (shift    (scenario-shorthand---PRC---weighted-address-bit-sum)    relof)    (call-instruction---callee---address-lo))
    ;;
    (eq!    (shift    (scenario-shorthand---PRC---failure)                     relof)    scenario/CALL_PRC_FAILURE)
    (eq!    (shift    scenario/PRC_SUCCESS_CALLER_WILL_REVERT                  relof)    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT)
    (eq!    (shift    scenario/PRC_SUCCESS_CALLER_WONT_REVERT                  relof)    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT)
    ;;
    (eq!    (shift    scenario/PRC_CALLER_GAS                                  relof)    (-    GAS_ACTUAL    (call-instruction---STP-gas-upfront)    (call-instruction---STP-gas-paid-out-of-pocket)))
    (eq!    (shift    scenario/PRC_CALLEE_GAS                                  relof)    (+    (call-instruction---STP-gas-paid-out-of-pocket)    (call-instruction---STP-call-stipend)))
    ;; gas owed to CALLER will be set later
    ;;
    (eq!    (shift    scenario/PRC_CDO                                         relof)    (call-instruction---type-safe-cdo))
    (eq!    (shift    scenario/PRC_CDS                                         relof)    (call-instruction---type-safe-cds))
    (eq!    (shift    scenario/PRC_RAO                                         relof)    (call-instruction---type-safe-r@o))
    (eq!    (shift    scenario/PRC_RAC                                         relof)    (call-instruction---type-safe-r@c))
    ))
