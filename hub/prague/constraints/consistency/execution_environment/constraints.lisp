(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                        ;;
;;    X.2 Execution environment consistency constraints   ;;
;;                                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint execution-environment-consistency---constancies ()
               (if-not-zero envcp_CN
                            (if-eq (next envcp_CN) envcp_CN
                                   (begin
                                     (will-remain-constant!   envcp_CFI          )
                                     (will-remain-constant!   envcp_CALLER_CN    )
                                     (will-remain-constant!   envcp_CN_WILL_REV  )
                                     (will-remain-constant!   envcp_CN_GETS_REV  )
                                     (will-remain-constant!   envcp_CN_SELF_REV  )
                                     (will-remain-constant!   envcp_CN_REV_STAMP )))))

(defconstraint execution-environment-consistency---linking ()
               (if-not-zero envcp_CN
                            (if-eq (next envcp_CN) envcp_CN
                                   (if-not (will-remain-constant! envcp_HUB_STAMP)
                                           (begin
                                             (eq! (next   envcp_PC)             envcp_PC_NEW)
                                             (eq! (next   envcp_HEIGHT)         envcp_HEIGHT_NEW)
                                             (eq! (next   envcp_GAS_EXPECTED)   envcp_GAS_NEXT))))))

(defconstraint execution-environment-consistency---initialization ()
               (if-not (will-remain-constant! envcp_CN)
                       (begin
                         (vanishes! (next envcp_PC))
                         (vanishes! (next envcp_HEIGHT)))))
