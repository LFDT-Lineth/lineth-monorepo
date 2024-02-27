(module hub_v2)

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
         kappa                                 ;; row offset
         cn                                    ;; context number
         caller                                ;; caller context
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
         call_data_offset                      ;; call data offset
         call_data_size                        ;; call data size
         return_at_offset                      ;; return at offset
         return_at_capacity                    ;; return at capacity
         )
  (begin
    (eq!       (shift context/CONTEXT_NUMBER                    kappa)         cn                             )
    (eq!       (shift context/CALLER_CONTEXT_NUMBER             kappa)         caller                         )
    (eq!       (shift context/CALL_STACK_DEPTH                  kappa)         csd                            )
    (eq!       (shift context/IS_ROOT                           kappa)         is_root                        )
    (eq!       (shift context/IS_STATIC                         kappa)         is_static                      )
    (eq!       (shift context/ACCOUNT_ADDRESS_HI                kappa)         account_address_hi             )
    (eq!       (shift context/ACCOUNT_ADDRESS_LO                kappa)         account_address_lo             )
    (eq!       (shift context/ACCOUNT_DEPLOYMENT_NUMBER         kappa)         account_deployment_number      )
    (eq!       (shift context/BYTE_CODE_ADDRESS_HI              kappa)         byte_code_address_hi           )
    (eq!       (shift context/BYTE_CODE_ADDRESS_LO              kappa)         byte_code_address_lo           )
    (eq!       (shift context/BYTE_CODE_DEPLOYMENT_NUMBER       kappa)         byte_code_deployment_number    )
    (eq!       (shift context/BYTE_CODE_DEPLOYMENT_STATUS       kappa)         byte_code_deployment_status    )
    (eq!       (shift context/BYTE_CODE_CODE_FRAGMENT_INDEX     kappa)         byte_code_cfi                  )
    (eq!       (shift context/CALLER_ADDRESS_HI                 kappa)         caller_address_hi              )
    (eq!       (shift context/CALLER_ADDRESS_LO                 kappa)         caller_address_lo              )
    (eq!       (shift context/CALL_VALUE                        kappa)         call_value                     )
    (eq!       (shift context/CALL_DATA_OFFSET                  kappa)         call_data_offset               )
    (eq!       (shift context/CALL_DATA_SIZE                    kappa)         call_data_size                 )
    (eq!       (shift context/RETURN_AT_OFFSET                  kappa)         return_at_offset               )
    (eq!       (shift context/RETURN_AT_CAPACITY                kappa)         return_at_capacity             )
    (vanishes! (shift context/UPDATE                            kappa))
    (vanishes! (shift context/RETURNER_CONTEXT_NUMBER           kappa))
    (vanishes! (shift context/RETURN_DATA_OFFSET                kappa))
    (vanishes! (shift context/RETURN_DATA_SIZE                  kappa))
    )
  )

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   6.3 Specialized constraints   ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (read-context-data
         kappa              ;; row offset
         context_number     ;; context to read
         ) (begin
         (eq!      (shift context/CONTEXT_NUMBER kappa) context_number)
         (vanishes (shift context/UPDATE         kappa)               )))

(defun (provide-return-data 
         kappa                               ;; offset
         return_data_receiver_context        ;; receiver context
         return_data_provider_context        ;; provider context
         return_data_offset                  ;; rdo
         return_data_size                    ;; rds
         ) (begin
         (eq! (shift context/UPDATE                      kappa)        1                            )
         (eq! (shift context/CONTEXT_NUMBER              kappa)        return_data_receiver_context )
         (eq! (shift context/RETURNER_CONTEXT_NUMBER     kappa)        return_data_provider_context )
         (eq! (shift context/RETURN_DATA_OFFSET          kappa)        return_data_offset           )
         (eq! (shift context/RETURN_DATA_SIZE            kappa)        return_data_size             )))

(defun (execution-provides-empty-return-data kappa)
  (provide-return-data
    kappa
    CALLER_CONTEXT_NUMBER
    CONTEXT_NUMBER
    0
    0))

(defun (nonexecution-provides-empty-return-data kappa)
  (provide-return-data
    kappa
    CONTEXT_NUMBER
    (+ 1 HUB_STAMP)
    0
    0))
