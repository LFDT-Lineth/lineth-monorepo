(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                   ;;
;;    X.Y.Z.2 initializeCalleeContext constraints    ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (initialize-callee-context    relative_row_offset)
  (initialize-context
         relative_row_offset                                                                      ;; row offset
         CONTEXT_NUMBER_NEW                                                                       ;; context number
         (+    1    (call-instruction---current-call-stack-depth))                                ;; call stack depth
         0                                                                                        ;; is root
         (call-instruction---new-context-is-static)                                               ;; is static
         (call-instruction---new-account-address-hi)                                              ;; account address high
         (call-instruction---new-account-address-lo)                                              ;; account address low
         (call-instruction---new-account-deployment-number)                                       ;; account deployment number
         (call-instruction---callee-address-hi)                                                   ;; byte code address high
         (call-instruction---callee-address-lo)                                                   ;; byte code address low
         (shift    account/DEPLOYMENT_NUMBER    CALL_1st_callee_account_row___row_offset)         ;; byte code deployment number
         (shift    account/DEPLOYMENT_STATUS    CALL_1st_callee_account_row___row_offset)         ;; byte code deployment status
         (call-instruction---callee-code-fragment-index)                                          ;; byte code code fragment index
         (call-instruction---new-caller-address-hi)                                               ;; caller address high
         (call-instruction---new-caller-address-lo)                                               ;; caller address low
         (call-instruction---new-call-value)                                                      ;; call value
         CONTEXT_NUMBER                                                                           ;; caller context
         (call-instruction---type-safe-cdo)                                                       ;; call data offset
         (call-instruction---type-safe-cds)                                                       ;; call data size
         (call-instruction---type-safe-r@o)                                                       ;; return at offset
         (call-instruction---type-safe-r@c)                                                       ;; return at capacity
         )
  )
