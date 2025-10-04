(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X.Y Constancy conditions   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; the following constraint allows to express stamp constancy for stamps that may only increment by 0 or 1
(defun (constancy-wrt-0-1-increments-stamp stamp  col)
  (if-not (did-inc! stamp 1)
          (remained-constant! col)))

;; usecases thereof
(defun (block-constancy        col) (constancy-wrt-0-1-increments-stamp    BLK_NUMBER         col))
(defun (transaction-constancy  col) (constancy-wrt-0-1-increments-stamp    TOTL_TXN_NUMBER    col))
(defun (hub-stamp-constancy    col) (constancy-wrt-0-1-increments-stamp    HUB_STAMP          col))
(defun (stack-row-constancy    col) (if-not-zero PEEK_AT_STACK
                                                 (if-not-zero     COUNTER_TLI
                                                                  (remained-constant!    col))))
(defun (context-constancy      col) (if (remained-constant! CN) (remained-constant!    col)))
