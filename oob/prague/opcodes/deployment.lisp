(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   OOB_INST_DEPLOYMENT   ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; Note. Here "DEPLOYMENT" refers to the execution of the RETURN opcode in a deployment context

(defun (deployment---standard-precondition)            IS_DEPLOYMENT)
(defun (deployment---code-size-hi)                     [DATA 1])
(defun (deployment---code-size-lo)                     [DATA 2])
(defun (deployment---max-code-size-exception)          [DATA 7])
(defun (deployment---exceeds-max-code-size)            OUTGOING_RES_LO)

(defconstraint deployment---compare-max-code-size-against-code-size (:guard (* (assumption---fresh-new-stamp) (deployment---standard-precondition)))
  (call-to-LT 0 0 MAX_CODE_SIZE (deployment---code-size-hi) (deployment---code-size-lo)))

(defconstraint deployment---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (deployment---standard-precondition)))
  (eq! (deployment---max-code-size-exception) (deployment---exceeds-max-code-size)))
