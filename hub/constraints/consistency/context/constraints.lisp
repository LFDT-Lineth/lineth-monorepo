(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    X.3 Context consistency constraints   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint context-consistency---perm-cn-first-and-cn-again-constraints ()
               (begin
                 (eq!   (+    con_AGAIN    con_FIRST)
                        ccp_PEEK_AT_CONTEXT)
                 (if-zero    (force-bool ccp_PEEK_AT_CONTEXT)
                             (eq!    (next con_FIRST)    (next ccp_PEEK_AT_CONTEXT)))
                 (if-not-zero  ccp_PEEK_AT_CONTEXT
                               (if-not-zero (next    ccp_PEEK_AT_CONTEXT)
                                            (if-not-zero    (will-remain-constant!   ccp_CONTEXT_NUMBER)
                                                            (will-eq! con_FIRST 1)
                                                            (will-eq! con_AGAIN 1))))))

(defconstraint context-consistency---context-data-immutability ()
               (if-not-zero (next con_AGAIN)
                            (if-not-zero (next ccp_CONTEXT_NUMBER)
                                         (begin
                                           ( will-remain-constant!  ccp_CALL_STACK_DEPTH              )
                                           ( will-remain-constant!  ccp_IS_ROOT                       )
                                           ( will-remain-constant!  ccp_IS_STATIC                     )
                                           ( will-remain-constant!  ccp_ACCOUNT_ADDRESS_HI            )
                                           ( will-remain-constant!  ccp_ACCOUNT_ADDRESS_LO            )
                                           ( will-remain-constant!  ccp_ACCOUNT_DEPLOYMENT_NUMBER     )
                                           ( will-remain-constant!  ccp_BYTE_CODE_ADDRESS_HI          )
                                           ( will-remain-constant!  ccp_BYTE_CODE_ADDRESS_LO          )
                                           ( will-remain-constant!  ccp_BYTE_CODE_DEPLOYMENT_NUMBER   )
                                           ( will-remain-constant!  ccp_BYTE_CODE_DEPLOYMENT_STATUS   )
                                           ( will-remain-constant!  ccp_BYTE_CODE_CODE_FRAGMENT_INDEX )
                                           ( will-remain-constant!  ccp_CALLER_ADDRESS_HI             )
                                           ( will-remain-constant!  ccp_CALLER_ADDRESS_LO             )
                                           ( will-remain-constant!  ccp_CALL_VALUE                    )
                                           ( will-remain-constant!  ccp_CALL_DATA_CONTEXT_NUMBER      )
                                           ( will-remain-constant!  ccp_CALL_DATA_OFFSET              )
                                           ( will-remain-constant!  ccp_CALL_DATA_SIZE                )
                                           ( will-remain-constant!  ccp_RETURN_AT_OFFSET              )
                                           ( will-remain-constant!  ccp_RETURN_AT_CAPACITY            )))))

(defconstraint context-consistency---context-data-return-data-constancy ()
               (if-not-zero (next con_AGAIN)
                            (if-not-zero (next ccp_CONTEXT_NUMBER)
                                         (if-zero (force-bool (next ccp_UPDATE))
                                                  (begin
                                                    ( will-remain-constant!  ccp_RETURN_DATA_CONTEXT_NUMBER )
                                                    ( will-remain-constant!  ccp_RETURN_DATA_OFFSET         )
                                                    ( will-remain-constant!  ccp_RETURN_DATA_SIZE           ))))))
