(defun
  (mxp-to-euc-selector)
  (* mxp.COMPUTATION mxp.computation/EUC_FLAG)
  )

(deflookup
  mxp-into-euc
  ;reference columns
  (
    euc.DIVIDEND
    euc.DIVISOR
    euc.QUOTIENT
    euc.CEIL
    euc.DONE
  )
  ;source columns
  (
    (* mxp.computation/ARG_1_LO (mxp-to-euc-selector))
    (* mxp.computation/ARG_2_LO (mxp-to-euc-selector))
    (* mxp.computation/RES_A    (mxp-to-euc-selector))
    (* mxp.computation/RES_B    (mxp-to-euc-selector))
    (mxp-to-euc-selector)
  ))
