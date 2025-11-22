(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                                                    ;;
;;   For POINT_EVALUATION, BLS_G1_ADD, BLS_G2_ADD, BLS_MAP_FP_TO_G1, BLS_MAP_FP2_TO_G2, P256_VERIFY   ;;
;;                                                                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc---fixed-size---cancun-prague-osaka---standard-precondition)   (+   (flag-sum-cancun-precompiles)
                                                                               (flag-sum-prague-precompiles-fixed-size)
                                                                               (flag-sum-osaka-precompiles)
                                                                               ))

(defun (fixed-cds)   (+  (*   PRECOMPILE_CALL_DATA_SIZE___POINT_EVALUATION   IS_POINT_EVALUATION  )
                         (*   PRECOMPILE_CALL_DATA_SIZE___G1_ADD             IS_BLS_G1_ADD        )
                         (*   PRECOMPILE_CALL_DATA_SIZE___G2_ADD             IS_BLS_G2_ADD        )
                         (*   PRECOMPILE_CALL_DATA_SIZE___FP_TO_G1           IS_BLS_MAP_FP_TO_G1  )
                         (*   PRECOMPILE_CALL_DATA_SIZE___FP2_TO_G2          IS_BLS_MAP_FP2_TO_G2 )
                         (*   PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY        IS_P256_VERIFY       )
                         ))

(defun (fixed-gast-cost)   (+  (*   GAS_CONST_POINT_EVALUATION    IS_POINT_EVALUATION  )
                               (*   GAS_CONST_BLS_G1_ADD          IS_BLS_G1_ADD        )
                               (*   GAS_CONST_BLS_G2_ADD          IS_BLS_G2_ADD        )
                               (*   GAS_CONST_BLS_MAP_FP_TO_G1    IS_BLS_MAP_FP_TO_G1  )
                               (*   GAS_CONST_BLS_MAP_FP2_TO_G2   IS_BLS_MAP_FP2_TO_G2 )
                               (*   GAS_CONST_P256_VERIFY         IS_P256_VERIFY       )
                               ))

(defun (prc---fixed-size---cancun-prague-osaka---precompile-cost)          (fixed-gast-cost))
(defun (prc---fixed-size---cancun-prague-osaka---valid-cds)                (shift OUTGOING_RES_LO 2))
(defun (prc---fixed-size---cancun-prague-osaka---sufficient-gas)           (- 1 (shift OUTGOING_RES_LO 3)))
(defun (p256-verify-valid-cds)                                             (prc---fixed-size---cancun-prague-osaka---valid-cds))

(defconstraint   prc---fixed-size---cancun-prague-osaka---check-cds-validity
                 (:guard (* (assumption---fresh-new-stamp) (prc---fixed-size---cancun-prague-osaka---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (call-to-EQ   2
                               0
                               (prc---cds)
                               0
                               (fixed-cds)
                               ))

(defconstraint   prc---fixed-size---cancun-prague-osaka---compare-call-gas-against-precompile-cost
                 (:guard (* (assumption---fresh-new-stamp) (prc---fixed-size---cancun-prague-osaka---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (call-to-LT   3
                               0
                               (prc---callee-gas)
                               0
                               (prc---fixed-size---cancun-prague-osaka---precompile-cost)
                               ))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;   Justifying HUB predictions   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   prc---fixed-size---cancun-prague-osaka---justifying-hub-predictions---setting-hub-success---cancun-prague
                 (:guard (* (assumption---fresh-new-stamp) (prc---fixed-size---cancun-prague-osaka---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (+ (flag-sum-cancun-precompiles) (flag-sum-prague-precompiles-fixed-size))
                                (eq!   (prc---hub-success)
                                       (* (prc---fixed-size---cancun-prague-osaka---valid-cds)
                                          (prc---fixed-size---cancun-prague-osaka---sufficient-gas)))))


(defconstraint   prc---fixed-size---cancun-prague-osaka---justifying-hub-predictions---setting-hub-success---osaka
                 (:guard (* (assumption---fresh-new-stamp) (prc---fixed-size---cancun-prague-osaka---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (flag-sum-osaka-precompiles)
                                (eq!   (prc---hub-success)
                                       (prc---fixed-size---cancun-prague-osaka---sufficient-gas))))


(defconstraint   prc---fixed-size---cancun-prague-osaka---justifying-hub-predictions---setting-return-gas
                 (:guard (* (assumption---fresh-new-stamp) (prc---fixed-size---cancun-prague-osaka---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero       (prc---hub-success)
                                (vanishes! (prc---return-gas))
                                (eq!       (prc---return-gas)
                                           (- (prc---callee-gas) (prc---fixed-size---cancun-prague-osaka---precompile-cost)))))

