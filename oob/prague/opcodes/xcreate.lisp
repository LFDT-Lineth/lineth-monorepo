(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                      ;;
;;   OOB_INST_XCREATE   ;;
;;                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (xcreate---standard-precondition)                       IS_XCREATE      )
(defun (xcreate---init-code-size-hi)                           [DATA  1]       )
(defun (xcreate---init-code-size-lo)                           [DATA  2]       ) ;; ""
(defun (xcreate---init-code-size-exceeds-MAX_INIT_CODE_SIZE)   OUTGOING_RES_LO )


(defconstraint   xcreate---compare-init-code-size-against-MAX_INIT_CODE_SIZE
                 (:guard (* (assumption---fresh-new-stamp) (xcreate---standard-precondition)))
                 (call-to-LT   0
                               0
                               MAX_INIT_CODE_SIZE
                               (xcreate---init-code-size-hi)
                               (xcreate---init-code-size-lo)))

(defconstraint   xcreate---enforce-maxcsx
                 (:guard (* (assumption---fresh-new-stamp) (xcreate---standard-precondition)))
                 (eq!   (xcreate---init-code-size-exceeds-MAX_INIT_CODE_SIZE)   1))
