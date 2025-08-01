(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;   OOB_INST_MODEXP_xbs  ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-xbs---standard-precondition)    IS_MODEXP_XBS)
(defun (prc-modexp-xbs---xbs-hi)                   [DATA 1])
(defun (prc-modexp-xbs---xbs-lo)                   [DATA 2])
(defun (prc-modexp-xbs---ybs-lo)                   [DATA 3])
(defun (prc-modexp-xbs---compute-max)              [DATA 4])
(defun (prc-modexp-xbs---max-xbs-ybs)              [DATA 7])
(defun (prc-modexp-xbs---xbs-nonzero)              [DATA 8])
(defun (prc-modexp-xbs---compo-to_512)             OUTGOING_RES_LO)
(defun (prc-modexp-xbs---comp)                     (next OUTGOING_RES_LO))

(defconstraint prc-modexp-xbs---compare-xbs-hi-against-513 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (call-to-LT 0 (prc-modexp-xbs---xbs-hi) (prc-modexp-xbs---xbs-lo) 0 513))

(defconstraint prc-modexp-xbs---compare-xbs-against-ybs (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (call-to-LT 1 0 (prc-modexp-xbs---xbs-lo) 0 (prc-modexp-xbs---ybs-lo)))

(defconstraint prc-modexp-xbs---check-xbs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (call-to-ISZERO 2 0 (prc-modexp-xbs---xbs-lo)))

(defconstraint additional-prc-modexp-xbs (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (begin (or! (eq! 0 (prc-modexp-xbs---compute-max)) (eq! 1 (prc-modexp-xbs---compute-max)))
         (eq! (prc-modexp-xbs---compo-to_512) 1)))

(defconstraint prc-modexp-xbs---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (if-zero (prc-modexp-xbs---compute-max)
           (begin (vanishes! (prc-modexp-xbs---max-xbs-ybs))
                  (vanishes! (prc-modexp-xbs---xbs-nonzero)))
           (begin (eq! (prc-modexp-xbs---xbs-nonzero)
                       (- 1 (shift OUTGOING_RES_LO 2)))
                  (if-zero (prc-modexp-xbs---comp)
                           (eq! (prc-modexp-xbs---max-xbs-ybs) (prc-modexp-xbs---xbs-lo))
                           (eq! (prc-modexp-xbs---max-xbs-ybs) (prc-modexp-xbs---ybs-lo))))))
