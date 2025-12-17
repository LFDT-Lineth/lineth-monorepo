(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;   OOB_INST_CREATE   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (create---standard-precondition)     IS_CREATE)
(defun (create---value-hi)                  [DATA  1])
(defun (create---value-lo)                  [DATA  2])
(defun (create---balance)                   [DATA  3])
(defun (create---nonce)                     [DATA  4])
(defun (create---has-code)                  [DATA  5])
(defun (create---call-stack-depth)          [DATA  6])
(defun (create---aborting-condition)        [DATA  7])
(defun (create---failure-condition)         [DATA  8])
(defun (create---creator-nonce)             [DATA  9])
(defun (create---init-code-size)            [DATA 10]) ;; ""


(defconstraint create---compare-balance-against-value
               (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
               (call-to-LT   0
                             0
                             (create---balance)
                             (create---value-hi)
                             (create---value-lo)))

(defun (create---insufficient-balance-abort) (shift OUTGOING_RES_LO 0))



(defconstraint create---compare-call-stack-depth-against-1024
               (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
               (call-to-LT   1
                             0
                             (create---call-stack-depth)
                             0
                             1024))

(defun (create---stack-depth-abort) (- 1 (shift OUTGOING_RES_LO 1)))



(defconstraint create---check-nonce-is-zero
               (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
               (call-to-ISZERO   2
                                 0
                                 (create---nonce)))

(defun (create---nonzero-nonce) (- 1 (shift OUTGOING_RES_LO 2)))



(defconstraint create---compare-creator-nonce-against-max-nonce
               (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
               (call-to-LT   3
                             0
                             (create---creator-nonce)
                             0
                             EIP2681_MAX_NONCE))

(defun (create---creator-nonce-abort) (- 1 (shift OUTGOING_RES_LO 3)))



(defconstraint create---compare-init-code-size-to-MAX_INIT_CODE_SIZE
               (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
               (call-to-LT   4
                             0
                             MAX_INIT_CODE_SIZE
                             0
                             (create---init-code-size)
                             ))

(defun (create---init-code-size-exceeds-max-init-code-size) (shift OUTGOING_RES_LO 4))



(defun (create---aborting-conditions-sum)   (+   (create---insufficient-balance-abort)
                                                 (create---stack-depth-abort)
                                                 (create---creator-nonce-abort)))

(defconstraint create---justify-hub-predictions
               (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
               (begin    (vanishes! (create---init-code-size-exceeds-max-init-code-size))
                         (if-zero   (create---aborting-conditions-sum)
                                    (eq! (create---aborting-condition) 0)
                                    (eq! (create---aborting-condition) 1))
                         (eq! (create---failure-condition)
                              (* (- 1 (create---aborting-condition))
                                 (+ (create---has-code)
                                    (* (- 1 (create---has-code)) (create---nonzero-nonce)))))))
