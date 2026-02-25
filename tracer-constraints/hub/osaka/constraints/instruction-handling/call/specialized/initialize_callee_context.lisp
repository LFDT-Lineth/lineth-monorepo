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
         (+    1    (call-instruction---current-frame---call-stack-depth))                        ;; call stack depth
         0                                                                                        ;; is root
         (call-instruction---child-frame---is-static)                                             ;; is static
         (call-instruction---child-frame---account-address-hi)                                    ;; account address high
         (call-instruction---child-frame---account-address-lo)                                    ;; account address low
         (call-instruction---child-frame---account-deployment-number)                             ;; account deployment number
         (call-instruction---delegate-or-callee---address-hi)                                     ;; byte code address high
         (call-instruction---delegate-or-callee---address-lo)                                     ;; byte code address low
         (call-instruction---delegate-or-callee---deployment-number)                              ;; byte code deployment number
         (call-instruction---delegate-or-callee---deployment-status)                              ;; byte code deployment status
         (call-instruction---delegate-or-callee---cfi)                                            ;; byte code code fragment index
         (call-instruction---child-frame---caller-address-hi)                                     ;; caller address high
         (call-instruction---child-frame---caller-address-lo)                                     ;; caller address low
         (call-instruction---child-frame---call-value)                                            ;; call value
         CONTEXT_NUMBER                                                                           ;; caller context
         (call-instruction---type-safe-cdo)                                                       ;; call data offset
         (call-instruction---type-safe-cds)                                                       ;; call data size
         (call-instruction---type-safe-r@o)                                                       ;; return at offset
         (call-instruction---type-safe-r@c)                                                       ;; return at capacity
         )
  )

