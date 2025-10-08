(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;   For BLS_PAIRING_CHECK   ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-blspairingcheck---standard-precondition)                                   IS_BLS_PAIRING_CHECK)
(defun (prc-blspairingcheck---remainder)                                               (shift OUTGOING_RES_LO 2))
(defun (prc-blspairingcheck---cds-is-multiple-of-bls-pairing-check-pair-size)          (shift OUTGOING_RES_LO 3))
(defun (prc-blspairingcheck---valid-cds)                                               (* (prc---cds-is-non-zero) (prc-blspairingcheck---cds-is-multiple-of-bls-pairing-check-pair-size)))
(defun (prc-blspairingcheck---insufficient-gas)                                        (shift OUTGOING_RES_LO 4))
(defun (prc-blspairingcheck---sufficient-gas)                                          (- 1 (prc-blspairingcheck---insufficient-gas)))
(defun (prc-blspairingcheck---precompile-cost_PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK)       
                                                                  (+ (* GAS_CONST_BLS_PAIRING_CHECK PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK) (* GAS_CONST_BLS_PAIRING_CHECK_PAIR (prc---cds))))

(defconstraint prc-blspairingcheck---mod-cds-by-PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK (:guard (* (assumption---fresh-new-stamp) (prc-blspairingcheck---standard-precondition)))
  (call-to-MOD 2 0 (prc---cds) 0 PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK))

(defconstraint prc-blspairingcheck---check-remainder-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-blspairingcheck---standard-precondition)))
  (call-to-ISZERO 3 0 (prc-blspairingcheck---remainder)))

(defconstraint prc-blspairingcheck---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-blspairingcheck---standard-precondition)))
  (if-zero (prc-blspairingcheck---valid-cds)
           (noCall 4)
           (begin (vanishes! (shift ADD_FLAG 4))
                  (vanishes! (shift MOD_FLAG 4))
                  (eq! (shift WCP_FLAG 4) 1)
                  (vanishes! (shift BLS_REF_TABLE_FLAG 4))
                  (eq! (shift OUTGOING_INST 4) EVM_INST_LT)
                  (vanishes! (shift [OUTGOING_DATA 1] 4))
                  (eq! (shift [OUTGOING_DATA 2] 4) (prc---callee-gas))
                  (vanishes! (shift [OUTGOING_DATA 3] 4))
                  (eq! (* (shift [OUTGOING_DATA 4] 4) PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK)
                       (prc-blspairingcheck---precompile-cost_PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK)))))

(defconstraint prc-blspairingcheck---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-blspairingcheck---standard-precondition)))
  (begin (eq! (prc---hub-success)
              (* (prc-blspairingcheck---valid-cds) (prc-blspairingcheck---sufficient-gas)))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (* (prc---return-gas) PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK)
                       (- (* (prc---callee-gas) PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK) (prc-blspairingcheck---precompile-cost_PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK))))))
