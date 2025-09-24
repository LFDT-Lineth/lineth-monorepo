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


(defun    (precompile-scenario-row-setting    relative_row_offset)
  (begin
    (eq!    (shift    PEEK_AT_SCENARIO                                         relative_row_offset)    1)
    (eq!    (shift    (scenario-shorthand---PRC---sum)                         relative_row_offset)    1)
    (eq!    (shift    (scenario-shorthand---PRC---weighted-address-bit-sum)    relative_row_offset)    (call-instruction---callee-address-lo))
    ;;
    (eq!    (shift    (scenario-shorthand---PRC---failure)                     relative_row_offset)    scenario/CALL_PRC_FAILURE)
    (eq!    (shift    scenario/PRC_SUCCESS_CALLER_WILL_REVERT                  relative_row_offset)    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT)
    (eq!    (shift    scenario/PRC_SUCCESS_CALLER_WONT_REVERT                  relative_row_offset)    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT)
    ;;
    (eq!    (shift    scenario/PRC_CALLER_GAS                                  relative_row_offset)    (-    GAS_ACTUAL    (call-instruction---STP-gas-upfront)    (call-instruction---STP-gas-paid-out-of-pocket)))
    (eq!    (shift    scenario/PRC_CALLEE_GAS                                  relative_row_offset)    (+    (call-instruction---STP-gas-paid-out-of-pocket)    (call-instruction---STP-call-stipend)))
    ;; gas owed to CALLER will be set later
    ;;
    (eq!    (shift    scenario/PRC_CDO                                         relative_row_offset)    (call-instruction---type-safe-cdo))
    (eq!    (shift    scenario/PRC_CDS                                         relative_row_offset)    (call-instruction---type-safe-cds))
    (eq!    (shift    scenario/PRC_RAO                                         relative_row_offset)    (call-instruction---type-safe-r@o))
    (eq!    (shift    scenario/PRC_RAC                                         relative_row_offset)    (call-instruction---type-safe-r@c))
    ))
