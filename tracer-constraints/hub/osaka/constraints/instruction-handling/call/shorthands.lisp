(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    X.Y.6 Shorthands   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (call-instruction---is-CALL---unshifted)                         [stack/DEC_FLAG  1]                                                 )
(defun    (call-instruction---is-CALLCODE---unshifted)                     [stack/DEC_FLAG  2]                                                 )
(defun    (call-instruction---is-CALL)                           (shift    [stack/DEC_FLAG  1]                 CALL_1st_stack_row___row_offset))
(defun    (call-instruction---is-CALLCODE)                       (shift    [stack/DEC_FLAG  2]                 CALL_1st_stack_row___row_offset))
(defun    (call-instruction---is-DELEGATECALL)                   (shift    [stack/DEC_FLAG  3]                 CALL_1st_stack_row___row_offset))
(defun    (call-instruction---is-STATICCALL)                     (shift    [stack/DEC_FLAG  4]                 CALL_1st_stack_row___row_offset)) ;; ""
(defun    (call-instruction---STACK-staticx)                     (shift    stack/STATICX                       CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-mxpx)                        (shift    stack/MXPX                          CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-oogx)                        (shift    stack/OOGX                          CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-instruction)                 (shift    stack/INSTRUCTION                   CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-cdo-hi)                      (shift    [stack/STACK_ITEM_VALUE_HI  1]      CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-cdo-lo)                      (shift    [stack/STACK_ITEM_VALUE_LO  1]      CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-cds-hi)                      (shift    [stack/STACK_ITEM_VALUE_HI  2]      CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-cds-lo)                      (shift    [stack/STACK_ITEM_VALUE_LO  2]      CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-r@o-hi)                      (shift    [stack/STACK_ITEM_VALUE_HI  3]      CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-r@o-lo)                      (shift    [stack/STACK_ITEM_VALUE_LO  3]      CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-r@c-hi)                      (shift    [stack/STACK_ITEM_VALUE_HI  4]      CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-r@c-lo)                      (shift    [stack/STACK_ITEM_VALUE_LO  4]      CALL_1st_stack_row___row_offset))
(defun    (call-instruction---STACK-gas-hi)                      (shift    [stack/STACK_ITEM_VALUE_HI  1]      CALL_2nd_stack_row___row_offset))
(defun    (call-instruction---STACK-gas-lo)                      (shift    [stack/STACK_ITEM_VALUE_LO  1]      CALL_2nd_stack_row___row_offset))
(defun    (call-instruction---STACK-raw-callee-address-hi)       (shift    [stack/STACK_ITEM_VALUE_HI  2]      CALL_2nd_stack_row___row_offset))
(defun    (call-instruction---STACK-raw-callee-address-lo)       (shift    [stack/STACK_ITEM_VALUE_LO  2]      CALL_2nd_stack_row___row_offset))
(defun    (call-instruction---STACK-value-hi)                    (shift    [stack/STACK_ITEM_VALUE_HI  3]      CALL_2nd_stack_row___row_offset))
(defun    (call-instruction---STACK-value-lo)                    (shift    [stack/STACK_ITEM_VALUE_LO  3]      CALL_2nd_stack_row___row_offset))
(defun    (call-instruction---STACK-success-bit-hi)              (shift    [stack/STACK_ITEM_VALUE_HI  4]      CALL_2nd_stack_row___row_offset))
(defun    (call-instruction---STACK-success-bit-lo)              (shift    [stack/STACK_ITEM_VALUE_LO  4]      CALL_2nd_stack_row___row_offset)) ;; ""

(defun    (call-instruction---gas-actual)                        (shift    GAS_ACTUAL                          CALL_2nd_stack_row___row_offset  ))

(defun    (call-instruction---current-frame---account-address-hi)        (shift    context/ACCOUNT_ADDRESS_HI          CALL_1st_context_row___row_offset))
(defun    (call-instruction---current-frame---account-address-lo)        (shift    context/ACCOUNT_ADDRESS_LO          CALL_1st_context_row___row_offset))
(defun    (call-instruction---current-frame---account-deployment-number) (shift    context/ACCOUNT_DEPLOYMENT_NUMBER   CALL_1st_context_row___row_offset))
(defun    (call-instruction---current-frame---context-is-static)         (shift    context/IS_STATIC                   CALL_1st_context_row___row_offset))
(defun    (call-instruction---current-frame---caller-address-hi)         (shift    context/CALLER_ADDRESS_HI           CALL_1st_context_row___row_offset))
(defun    (call-instruction---current-frame---caller-address-lo)         (shift    context/CALLER_ADDRESS_LO           CALL_1st_context_row___row_offset))
(defun    (call-instruction---current-frame---call-value)                (shift    context/CALL_VALUE                  CALL_1st_context_row___row_offset))
(defun    (call-instruction---current-frame---call-stack-depth)          (shift    context/CALL_STACK_DEPTH            CALL_1st_context_row___row_offset))

(defun    (call-instruction---MXP-memory-expansion-exception)    (shift    misc/MXP_MXPX                       CALL_misc_row___row_offset))
(defun    (call-instruction---MXP-memory-expansion-gas)          (shift    misc/MXP_GAS_MXP                    CALL_misc_row___row_offset))
(defun    (call-instruction---MXP-size-1-nonzero-and-no-mxpx)    (shift    misc/MXP_SIZE_1_NONZERO_NO_MXPX     CALL_misc_row___row_offset))
(defun    (call-instruction---MXP-size-2-nonzero-and-no-mxpx)    (shift    misc/MXP_SIZE_2_NONZERO_NO_MXPX     CALL_misc_row___row_offset))
(defun    (call-instruction---STP-gas-upfront)                   (shift    misc/STP_GAS_UPFRONT_GAS_COST       CALL_misc_row___row_offset))
(defun    (call-instruction---STP-gas-paid-out-of-pocket)        (shift    misc/STP_GAS_PAID_OUT_OF_POCKET     CALL_misc_row___row_offset))
(defun    (call-instruction---STP-call-stipend)                  (shift    misc/STP_GAS_STIPEND                CALL_misc_row___row_offset))
(defun    (call-instruction---STP-out-of-gas-exception)          (shift    misc/STP_OOGX                       CALL_misc_row___row_offset))
(defun    (call-instruction---OOB-nonzero-value)                 (shift    [misc/OOB_DATA  7]                  CALL_misc_row___row_offset))
(defun    (call-instruction---OOB-aborting-condition)            (shift    [misc/OOB_DATA  8]                  CALL_misc_row___row_offset)) ;; ""

(defun    (call-instruction---caller---balance)                    (shift    account/BALANCE                     CALL_1st_caller_account_row___row_offset))

(defun    (call-instruction---callee---address-hi)                 (shift    account/ADDRESS_HI                  CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---address-lo)                 (shift    account/ADDRESS_LO                  CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---deployment-number)          (shift    account/DEPLOYMENT_NUMBER           CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---code-fragment-index)        (shift    account/CODE_FRAGMENT_INDEX         CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---has-code)                   (shift    account/HAS_CODE                    CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---warmth)                     (shift    account/WARMTH                      CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---exists)                     (shift    account/EXISTS                      CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---is-precompile)              (shift    account/IS_PRECOMPILE               CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---is-delegated)               (shift    account/IS_DELEGATED                CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---delegate-address-hi)        (shift    account/DELEGATION_ADDRESS_HI       CALL_1st_callee_account_row___row_offset))
(defun    (call-instruction---callee---delegate-address-lo)        (shift    account/DELEGATION_ADDRESS_LO       CALL_1st_callee_account_row___row_offset))

(defun    (call-instruction---callee---is-delegated-to-self)       (if-zero  (call-instruction---callee---is-delegated)
                                                                             ;; callee_is_delegated ≡ false
                                                                             0
                                                                             ;; callee_is_delegated ≡ true
                                                                             (if-eq-else   (call-instruction---callee---delegate-address-hi)  (call-instruction---callee---address-hi)
                                                                                           (if-eq-else   (call-instruction---callee---delegate-address-lo)  (call-instruction---callee---address-lo)
                                                                                                         1
                                                                                                         0
                                                                                                         )
                                                                                           0
                                                                                           )))

(defun    (call-instruction---delegate-or-callee---address-hi          )    (shift account/ADDRESS_HI             CALL_1st_delegt_account_row___row_offset ))
(defun    (call-instruction---delegate-or-callee---address-lo          )    (shift account/ADDRESS_LO             CALL_1st_delegt_account_row___row_offset ))
(defun    (call-instruction---delegate-or-callee---deployment-number   )    (shift account/DEPLOYMENT_NUMBER      CALL_1st_delegt_account_row___row_offset ))
(defun    (call-instruction---delegate-or-callee---deployment-status   )    (shift account/DEPLOYMENT_STATUS      CALL_1st_delegt_account_row___row_offset ))
(defun    (call-instruction---delegate-or-callee---cfi                 )    (shift account/CODE_FRAGMENT_INDEX    CALL_1st_delegt_account_row___row_offset ))
(defun    (call-instruction---delegate-or-callee---has-code            )    (shift account/HAS_CODE               CALL_1st_delegt_account_row___row_offset ))
(defun    (call-instruction---delegate-or-callee---warmth              )    (shift account/WARMTH                 CALL_1st_delegt_account_row___row_offset ))
(defun    (call-instruction---delegate-or-callee---exists              )    (shift account/EXISTS                 CALL_1st_delegt_account_row___row_offset ))
(defun    (call-instruction---delegate-or-callee---is-delegated        )    (shift account/IS_DELEGATED           CALL_1st_delegt_account_row___row_offset ))

(defun    (call-instruction---is-delegate-warmth                       )    (if-zero   (call-instruction---callee---is-delegated)
                                                                                       0
                                                                                       (call-instruction---delegate-or-callee---warmth)))


;; revert data shorthands
(defun    (call-instruction---caller---will-revert)              (shift    CONTEXT_WILL_REVERT     CALL_1st_stack_row___row_offset))
(defun    (call-instruction---caller---revert-stamp)             (shift    CONTEXT_REVERT_STAMP    CALL_1st_stack_row___row_offset))
(defun    (call-instruction---callee---self-reverts)             (shift    misc/CCSR_FLAG          CALL_misc_row___row_offset))
(defun    (call-instruction---callee---revert-stamp)             (shift    misc/CCRS_STAMP         CALL_misc_row___row_offset))

;; type safe call data and return at segment values
(defun    (call-instruction---type-safe-cdo)    (*   (call-instruction---STACK-cdo-lo)   (call-instruction---MXP-size-1-nonzero-and-no-mxpx)))
(defun    (call-instruction---type-safe-cds)         (call-instruction---STACK-cds-lo)                                                       )
(defun    (call-instruction---type-safe-r@o)    (*   (call-instruction---STACK-r@o-lo)   (call-instruction---MXP-size-2-nonzero-and-no-mxpx)))
(defun    (call-instruction---type-safe-r@c)         (call-instruction---STACK-r@c-lo)                                                       )
