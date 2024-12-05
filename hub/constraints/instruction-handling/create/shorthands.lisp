(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    X.Y.6 Shorthands   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (create-instruction---instruction)                  (shift   stack/INSTRUCTION                 CREATE_first_stack_row___row_offset))
(defun    (create-instruction---is-CREATE)                    (shift   [stack/DEC_FLAG 1]                CREATE_first_stack_row___row_offset))
(defun    (create-instruction---is-CREATE2)                   (shift   [stack/DEC_FLAG 2]                CREATE_first_stack_row___row_offset))
(defun    (create-instruction---STACK-staticx)                (shift   stack/STATICX                     CREATE_first_stack_row___row_offset))
(defun    (create-instruction---STACK-mxpx)                   (shift   stack/MXPX                        CREATE_first_stack_row___row_offset))
(defun    (create-instruction---STACK-oogx)                   (shift   stack/OOGX                        CREATE_first_stack_row___row_offset))
(defun    (create-instruction---STACK-offset-hi)              (shift   [stack/STACK_ITEM_VALUE_HI 1]     CREATE_first_stack_row___row_offset))
(defun    (create-instruction---STACK-offset-lo)              (shift   [stack/STACK_ITEM_VALUE_LO 1]     CREATE_first_stack_row___row_offset))
(defun    (create-instruction---STACK-size-hi)                (shift   [stack/STACK_ITEM_VALUE_HI 2]     CREATE_first_stack_row___row_offset))
(defun    (create-instruction---STACK-size-lo)                (shift   [stack/STACK_ITEM_VALUE_LO 2]     CREATE_first_stack_row___row_offset))
(defun    (create-instruction---creator-will-revert)          (shift   CONTEXT_WILL_REVERT               CREATE_first_stack_row___row_offset))
(defun    (create-instruction---creator-revert-stamp)         (shift   CONTEXT_REVERT_STAMP              CREATE_first_stack_row___row_offset))
(defun    (create-instruction---HASHINFO-keccak-hi)           (shift   stack/HASH_INFO_KECCAK_HI         CREATE_first_stack_row___row_offset))
(defun    (create-instruction---HASHINFO-keccak-lo)           (shift   stack/HASH_INFO_KECCAK_LO         CREATE_first_stack_row___row_offset))
(defun    (create-instruction---init-code-size)               (shift   [stack/STACK_ITEM_VALUE_LO 2]     CREATE_first_stack_row___row_offset))
(defun    (create-instruction---STACK-salt-hi)                (shift   [stack/STACK_ITEM_VALUE_HI 2]     CREATE_second_stack_row___row_offset))
(defun    (create-instruction---STACK-salt-lo)                (shift   [stack/STACK_ITEM_VALUE_LO 2]     CREATE_second_stack_row___row_offset))
(defun    (create-instruction---STACK-value-hi)               (shift   [stack/STACK_ITEM_VALUE_HI 3]     CREATE_second_stack_row___row_offset))
(defun    (create-instruction---STACK-value-lo)               (shift   [stack/STACK_ITEM_VALUE_LO 3]     CREATE_second_stack_row___row_offset))
(defun    (create-instruction---STACK-output-hi)              (shift   [stack/STACK_ITEM_VALUE_HI 4]     CREATE_second_stack_row___row_offset))
(defun    (create-instruction---STACK-output-lo)              (shift   [stack/STACK_ITEM_VALUE_LO 4]     CREATE_second_stack_row___row_offset))
(defun    (create-instruction---creator-address-hi)           (shift   context/ACCOUNT_ADDRESS_HI        CREATE_current_context_row___row_offset))
(defun    (create-instruction---creator-address-lo)           (shift   context/ACCOUNT_ADDRESS_LO        CREATE_current_context_row___row_offset))
(defun    (create-instruction---current-context-is-static)    (shift   context/IS_STATIC                 CREATE_current_context_row___row_offset))
(defun    (create-instruction---current-context-csd)          (shift   context/CALL_STACK_DEPTH          CREATE_current_context_row___row_offset))
(defun    (create-instruction---createe-self-reverts)         (shift   misc/CCSR_FLAG                    CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---createe-revert-stamp)         (shift   misc/CCRS_STAMP                   CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---OOB-aborting-condition)       (shift   [misc/OOB_DATA 7]                 CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---OOB-failure-condition)        (shift   [misc/OOB_DATA 8]                 CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---MXP-mxpx)                     (shift   misc/MXP_MXPX                     CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---MXP-gas)                      (shift   misc/MXP_GAS_MXP                  CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---MXP-mtntop)                   (shift   misc/MXP_MTNTOP                   CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---STP-gas-paid-out-of-pocket)   (shift   misc/STP_GAS_PAID_OUT_OF_POCKET   CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---STP-oogx)                     (shift   misc/STP_OOGX                     CREATE_miscellaneous_row___row_offset))
(defun    (create-instruction---compute-createe-address)      (shift   account/RLPADDR_FLAG              CREATE_first_creator_account_row___row_offset))
(defun    (create-instruction---createe-address-hi)           (shift   account/RLPADDR_DEP_ADDR_HI       CREATE_first_creator_account_row___row_offset))
(defun    (create-instruction---createe-address-lo)           (shift   account/RLPADDR_DEP_ADDR_LO       CREATE_first_creator_account_row___row_offset))
(defun    (create-instruction---creator-nonce)                (shift   account/NONCE                     CREATE_first_creator_account_row___row_offset))
(defun    (create-instruction---creator-balance)              (shift   account/BALANCE                   CREATE_first_creator_account_row___row_offset))
(defun    (create-instruction---deployment-cfi)               (shift   account/CODE_FRAGMENT_INDEX       CREATE_first_createe_account_row___row_offset))
(defun    (create-instruction---ROMLEX-flag)                  (shift   account/ROMLEX_FLAG               CREATE_first_createe_account_row___row_offset))
