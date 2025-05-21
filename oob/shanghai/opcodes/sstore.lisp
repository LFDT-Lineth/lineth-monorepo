(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;   OOB_INST_SSTORE   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (sstore---standard-precondition)     IS_SSTORE)
(defun (sstore---gas)                       [DATA 5])
(defun (sstore---sstorex)                   [DATA 7])
(defun (sstore---sufficient-gas)            OUTGOING_RES_LO)

(defconstraint sstore---compare-g-call-stipend-against-gas (:guard (* (assumption---fresh-new-stamp) (sstore---standard-precondition)))
  (call-to-LT 0 0 GAS_CONST_G_CALL_STIPEND 0 (sstore---gas)))

(defconstraint sstore---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (sstore---standard-precondition)))
  (eq! (sstore---sstorex) (- 1 (sstore---sufficient-gas))))
