(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   6.1 Setting the next context number   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (next-context-is-new)
  (eq! CONTEXT_NUMBER_NEW
       (+ 1 HUB_STAMP)))

(defun (next-context-is-current)
  (eq! CONTEXT_NUMBER_NEW
       CONTEXT_NUMBER))

(defun (next-context-is-caller)
  (eq! CONTEXT_NUMBER_NEW
       CALLER_CONTEXT_NUMBER))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   6.1 Setting the next context number   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (initialize-context
         relative_row_offset                   ;; row offset
         cn                                    ;; context number
         csd                                   ;; call stack depth
         is_root                               ;; is root
         is_static                             ;; is static
         account_address_hi                    ;; account address high
         account_address_lo                    ;; account address low
         account_deployment_number             ;; account deployment number
         byte_code_address_hi                  ;; byte code address high
         byte_code_address_lo                  ;; byte code address low
         byte_code_deployment_number           ;; byte code deployment number
         byte_code_deployment_status           ;; byte code deployment status
         byte_code_cfi                         ;; byte code code fragment index
         caller_address_hi                     ;; caller address high
         caller_address_lo                     ;; caller address low
         call_value                            ;; call value
         call_data_context_number              ;; caller context
         call_data_offset                      ;; call data offset
         call_data_size                        ;; call data size
         return_at_offset                      ;; return at offset
         return_at_capacity                    ;; return at capacity
         )
  (begin
    (eq!       (shift context/CONTEXT_NUMBER                    relative_row_offset)         cn                             )
    (eq!       (shift context/CALL_DATA_CONTEXT_NUMBER          relative_row_offset)         call_data_context_number       )
    (eq!       (shift context/CALL_STACK_DEPTH                  relative_row_offset)         csd                            )
    (eq!       (shift context/IS_ROOT                           relative_row_offset)         is_root                        )
    (eq!       (shift context/IS_STATIC                         relative_row_offset)         is_static                      )
    (eq!       (shift context/ACCOUNT_ADDRESS_HI                relative_row_offset)         account_address_hi             )
    (eq!       (shift context/ACCOUNT_ADDRESS_LO                relative_row_offset)         account_address_lo             )
    (eq!       (shift context/ACCOUNT_DEPLOYMENT_NUMBER         relative_row_offset)         account_deployment_number      )
    (eq!       (shift context/BYTE_CODE_ADDRESS_HI              relative_row_offset)         byte_code_address_hi           )
    (eq!       (shift context/BYTE_CODE_ADDRESS_LO              relative_row_offset)         byte_code_address_lo           )
    (eq!       (shift context/BYTE_CODE_DEPLOYMENT_NUMBER       relative_row_offset)         byte_code_deployment_number    )
    (eq!       (shift context/BYTE_CODE_DEPLOYMENT_STATUS       relative_row_offset)         byte_code_deployment_status    )
    (eq!       (shift context/BYTE_CODE_CODE_FRAGMENT_INDEX     relative_row_offset)         byte_code_cfi                  )
    (eq!       (shift context/CALLER_ADDRESS_HI                 relative_row_offset)         caller_address_hi              )
    (eq!       (shift context/CALLER_ADDRESS_LO                 relative_row_offset)         caller_address_lo              )
    (eq!       (shift context/CALL_VALUE                        relative_row_offset)         call_value                     )
    (eq!       (shift context/CALL_DATA_OFFSET                  relative_row_offset)         call_data_offset               )
    (eq!       (shift context/CALL_DATA_SIZE                    relative_row_offset)         call_data_size                 )
    (eq!       (shift context/RETURN_AT_OFFSET                  relative_row_offset)         return_at_offset               )
    (eq!       (shift context/RETURN_AT_CAPACITY                relative_row_offset)         return_at_capacity             )
    (vanishes! (shift context/UPDATE                            relative_row_offset))
    (vanishes! (shift context/RETURN_DATA_CONTEXT_NUMBER        relative_row_offset))
    (vanishes! (shift context/RETURN_DATA_OFFSET                relative_row_offset))
    (vanishes! (shift context/RETURN_DATA_SIZE                  relative_row_offset))
    )
  )

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   6.3 Specialized constraints   ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (read-context-data     relative_row_offset   ;; row offset
                              context_number        ;; context to read
                              )
  (begin
    (eq!       (shift context/CONTEXT_NUMBER    relative_row_offset) context_number)
    (vanishes! (shift context/UPDATE            relative_row_offset)               )))

(defun (provide-return-data     relative_row_offset            ;; row offset
                                return_data_receiver_context   ;; receiver context
                                return_data_provider_context   ;; provider context
                                return_data_offset             ;; rdo
                                return_data_size               ;; rds
                                )
  (begin
    (eq! (shift context/UPDATE                      relative_row_offset)        1                            )
    (eq! (shift context/CONTEXT_NUMBER              relative_row_offset)        return_data_receiver_context )
    (eq! (shift context/RETURN_DATA_CONTEXT_NUMBER  relative_row_offset)        return_data_provider_context )
    (eq! (shift context/RETURN_DATA_OFFSET          relative_row_offset)        return_data_offset           )
    (eq! (shift context/RETURN_DATA_SIZE            relative_row_offset)        return_data_size             )))

(defun (execution-provides-empty-return-data    relative_row_offset)
  (provide-return-data    relative_row_offset
                          CALLER_CONTEXT_NUMBER
                          CONTEXT_NUMBER
                          0
                          0))

(defun (nonexecution-provides-empty-return-data    relative_row_offset)
  (provide-return-data     relative_row_offset
                           CONTEXT_NUMBER
                           (+ 1 HUB_STAMP)
                           0
                           0))
