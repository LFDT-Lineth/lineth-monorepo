(module oob)


;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;   OOB_INST_RDC   ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;

;; Note. We use rdc as a shorthand for RETURNDATACOPY

(defun (rdc---standard-precondition)    IS_RDC)
(defun (rdc---offset-hi)                [DATA 1])
(defun (rdc---offset-lo)                [DATA 2])
(defun (rdc---size-hi)                  [DATA 3])
(defun (rdc---size-lo)                  [DATA 4])
(defun (rdc---rds)                      [DATA 5])
(defun (rdc---rdcx)                     [DATA 7])
(defun (rdc---rdc-roob)                 (- 1 OUTGOING_RES_LO))
(defun (rdc---rdc-soob)                 (shift OUTGOING_RES_LO 2))

(defconstraint rdc---check-offset-is-zero (:guard (* (assumption---fresh-new-stamp) (rdc---standard-precondition)))
  (call-to-ISZERO 0 (rdc---offset-hi) (rdc---size-hi)))

(defconstraint rdc---add-offset-and-size (:guard (* (assumption---fresh-new-stamp) (rdc---standard-precondition)))
  (if-zero (rdc---rdc-roob)
           (call-to-ADD 1 0 (rdc---offset-lo) 0 (rdc---size-lo))
           (noCall 1)))

(defconstraint rdc---compare-offset-plus-size-against-rds (:guard (* (assumption---fresh-new-stamp) (rdc---standard-precondition)))
  (if-zero (rdc---rdc-roob)
           (begin (vanishes! (shift ADD_FLAG 2))
                  (vanishes! (shift MOD_FLAG 2))
                  (eq! (shift WCP_FLAG 2) 1)
                  (eq! (shift OUTGOING_INST 2) EVM_INST_GT)
                  (vanishes! (shift [OUTGOING_DATA 3] 2))
                  (eq! (shift [OUTGOING_DATA 4] 2) (rdc---rds)))
           (noCall 2)))

(defconstraint rdc---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (rdc---standard-precondition)))
  (eq! (rdc---rdcx)
       (+ (rdc---rdc-roob)
          (* (- 1 (rdc---rdc-roob)) (rdc---rdc-soob)))))

