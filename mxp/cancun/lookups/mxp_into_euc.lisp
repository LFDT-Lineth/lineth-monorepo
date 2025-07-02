(defun
  (mxp-to-euc-selector)
  (* mxp.COMPUTATION mxp.computation/EUC_FLAG)
  )

(defclookup
  mxp-into-euc
  ;; target columns
  (
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
    euc.CEIL
    euc.DONE
  )
  ;; source selector
  (mxp-to-euc-selector)
  ;; source columns
  (
    mxp.computation/ARG_1_LO
    mxp.computation/ARG_2_LO
    mxp.computation/RES_A
    mxp.computation/RES_B
    1
  ))
