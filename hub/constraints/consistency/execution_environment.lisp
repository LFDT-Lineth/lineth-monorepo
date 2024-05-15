(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                        ;;
;;    X.2 Execution environment consistency constraints   ;;
;;                                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defpermutation
  ;; permuted columns
  (
   exec_env_consistency_perm_CN
   exec_env_consistency_perm_HUB_STAMP
   exec_env_consistency_perm_CFI
   exec_env_consistency_perm_CALLER_CN
   exec_env_consistency_perm_CN_WILL_REV
   exec_env_consistency_perm_CN_GETS_REV
   exec_env_consistency_perm_CN_SELF_REV
   exec_env_consistency_perm_CN_REV_STAMP
   exec_env_consistency_perm_PC
   exec_env_consistency_perm_PC_NEW
   exec_env_consistency_perm_HEIGHT
   exec_env_consistency_perm_HEIGHT_NEW
   exec_env_consistency_perm_GAS_EXPECTED
   exec_env_consistency_perm_GAS_NEXT
   )
  ;; original columns
  (
   (+ CN )
   (+ HUB_STAMP )
   CFI
   CALLER_CN
   CN_WILL_REV
   CN_GETS_REV
   CN_SELF_REV
   CN_REV_STAMP
   PC
   PC_NEW
   HEIGHT
   HEIGHT_NEW
   GAS_EXPECTED
   GAS_NEXT
   )
  )

(defconstraint consistency-exec-env-constancies ()
               (if-not-zero exec_env_consistency_perm_CN
                            (if-eq (next exec_env_consistency_perm_CN) exec_env_consistency_perm_CN
                                   (begin
                                     (will-eq! exec_env_consistency_perm_CFI          exec_env_consistency_perm_CFI)
                                     (will-eq! exec_env_consistency_perm_CALLER_CN    exec_env_consistency_perm_CALLER_CN)
                                     (will-eq! exec_env_consistency_perm_CN_WILL_REV  exec_env_consistency_perm_CN_WILL_REV)
                                     (will-eq! exec_env_consistency_perm_CN_GETS_REV  exec_env_consistency_perm_CN_GETS_REV)
                                     (will-eq! exec_env_consistency_perm_CN_SELF_REV  exec_env_consistency_perm_CN_SELF_REV)
                                     (will-eq! exec_env_consistency_perm_CN_REV_STAMP exec_env_consistency_perm_CN_REV_STAMP)))))

(defconstraint consistency-exec-env-linking ()
               (if-not-zero exec_env_consistency_perm_CN
                            (if-eq (next exec_env_consistency_perm_CN) exec_env_consistency_perm_CN
                                   (if-not-zero (will-remain-constant! exec_env_consistency_perm_HUB_STAMP)
                                                (begin
                                                  (will-eq! exec_env_consistency_perm_PC           exec_env_consistency_perm_PC_NEW)
                                                  (will-eq! exec_env_consistency_perm_HEIGHT       exec_env_consistency_perm_HEIGHT_NEW)
                                                  (will-eq! exec_env_consistency_perm_GAS_EXPECTED exec_env_consistency_perm_GAS_NEXT))))))
                                                                       
(defconstraint consistency-exec-env-initialization ()
               (if-not-zero (will-remain-constant! exec_env_consistency_perm_HUB_STAMP)
                            (begin
                              (vanishes! (next exec_env_consistency_perm_PC))
                              (vanishes! (next exec_env_consistency_perm_HEIGHT)))))
