(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   3.2 Constancy conditions   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; the following constraint allows to express stamp constancy for stamps that may only increment by 0 or 1
(defun (constancy-wrt-0-1-increments-stamp stamp  col)
  (if-not-zero (did-inc! stamp 1)
               (remained-constant! col)))

;; usecases thereof
(defun (batch-constancy        col) (constancy-wrt-0-1-increments-stamp BATCH_NUMBER                col))
(defun (transaction-constancy  col) (constancy-wrt-0-1-increments-stamp ABSOLUTE_TRANSACTION_NUMBER col))
(defun (hub-stamp-constancy    col) (constancy-wrt-0-1-increments-stamp HUB_STAMP                   col))
(defun (stack-row-constancy    col) (if-not-zero PEEK_AT_STACK (if-not-zero COUNTER_TLI (remained-constant! col))))

;; TODO: remove PEEK_AT_STACK -- stack-row-constancy will be used on stack rows where this is already a precondition


(defconstraint transaction-constancies () 
               (begin
                 (transaction-constancy HUB_STAMP_TRANSACTION_END)))

(defconstraint hub-stamp-constancies () 
               (begin
                 (debug (hub-stamp-constancy CN))
                 (debug (hub-stamp-constancy CN_NEW))
                 (debug (hub-stamp-constancy PC))
                 (debug (hub-stamp-constancy PC_NEW))
                 (debug (hub-stamp-constancy GAS_EXPECTED))
                 (debug (hub-stamp-constancy GAS_ACTUAL))
                 (debug (hub-stamp-constancy GAS_COST))
                 (debug (hub-stamp-constancy GAS_NEXT))
                 (debug (hub-stamp-constancy REFUND_COUNTER))
                 (debug (hub-stamp-constancy REFUND_COUNTER_NEW))

                 (hub-stamp-constancy TLI)
                 (hub-stamp-constancy NSR)
                 (hub-stamp-constancy CMC)
                 (hub-stamp-constancy XAHOY)))

;; ;; @Olivier TODO context-constancy after reordering
;; ;; context constancies
;; (defconstraint context-constancies ()
;;                (begin
;;                  (context-constancy PERM_CODE_FRAGMENT_INDEX)
;;                  (context-constancy PERM_CN_WILL_REV)
;;                  (context-constancy PERM_CN_GETS_REV)
;;                  (context-constancy PERM_CN_SELF_REV)
;;                  (context-constancy PERM_CN_REV_STAMP)
;;                  ))
