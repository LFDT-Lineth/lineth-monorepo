(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   OOB_INST_MODEXP_cds   ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-cds---standard-precondition)    IS_MODEXP_CDS)
(defun (prc-modexp-cds---extract-bbs)              [DATA 3])
(defun (prc-modexp-cds---extract-ebs)              [DATA 4])
(defun (prc-modexp-cds---extract-mbs)              [DATA 5])

(defconstraint prc-modexp-cds---compare-0-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-cds---standard-precondition)))
  (call-to-LT 0 0 0 0 (prc---cds)))

(defconstraint prc-modexp-cds---compare-32-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-cds---standard-precondition)))
  (call-to-LT 1 0 32 0 (prc---cds)))

(defconstraint prc-modexp-cds---compare-64-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-cds---standard-precondition)))
  (call-to-LT 2 0 64 0 (prc---cds)))

(defconstraint prc-modexp-cds---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-cds---standard-precondition)))
  (begin (eq! (prc-modexp-cds---extract-bbs) OUTGOING_RES_LO)
         (eq! (prc-modexp-cds---extract-ebs) (next OUTGOING_RES_LO))
         (eq! (prc-modexp-cds---extract-mbs) (shift OUTGOING_RES_LO 2))))

