(defun (hub-into-mxp-trigger)
  (* hub.PEEK_AT_MISCELLANEOUS hub.misc/MXP_FLAG))

(defclookup
  (hub-into-mxp :unchecked)
  ;; target columns
  (
    mxp.STAMP
    mxp.CN
    mxp.INST
    mxp.MXPX
    mxp.DEPLOYS
    mxp.OFFSET_1_HI
    mxp.OFFSET_1_LO
    mxp.OFFSET_2_HI
    mxp.OFFSET_2_LO
    mxp.SIZE_1_HI
    mxp.SIZE_1_LO
    mxp.SIZE_2_HI
    mxp.SIZE_2_LO
    mxp.WORDS
    mxp.GAS_MXP
    mxp.MTNTOP
    mxp.SIZE_1_NONZERO_NO_MXPX
    mxp.SIZE_2_NONZERO_NO_MXPX
  )
  ;; source selector
  (hub-into-mxp-trigger)
  ;; source columns
  (
    hub.MXP_STAMP
    hub.CONTEXT_NUMBER
    hub.misc/MXP_INST
    hub.misc/MXP_MXPX
    hub.misc/MXP_DEPLOYS
    hub.misc/MXP_OFFSET_1_HI
    hub.misc/MXP_OFFSET_1_LO
    hub.misc/MXP_OFFSET_2_HI
    hub.misc/MXP_OFFSET_2_LO
    hub.misc/MXP_SIZE_1_HI
    hub.misc/MXP_SIZE_1_LO
    hub.misc/MXP_SIZE_2_HI
    hub.misc/MXP_SIZE_2_LO
    hub.misc/MXP_WORDS
    hub.misc/MXP_GAS_MXP
    hub.misc/MXP_MTNTOP
    hub.misc/MXP_SIZE_1_NONZERO_NO_MXPX
    hub.misc/MXP_SIZE_2_NONZERO_NO_MXPX
  ))
