(defun (hub-into-mxp-trigger)
  (* hub.PEEK_AT_MISCELLANEOUS hub.misc/MXP_FLAG))

(defclookup
  (hub-into-mxp :unchecked)
  ;; target columns
  (
    mxp.MACRO
    mxp.MXP_STAMP
    mxp.CN
    mxp.macro/INST
    mxp.macro/DEPLOYING
    mxp.macro/OFFSET_1_HI
    mxp.macro/OFFSET_1_LO
    mxp.macro/OFFSET_2_HI
    mxp.macro/OFFSET_2_LO
    mxp.macro/SIZE_1_HI
    mxp.macro/SIZE_1_LO
    mxp.macro/SIZE_2_HI
    mxp.macro/SIZE_2_LO
    mxp.macro/RES
    mxp.macro/MXPX
    mxp.macro/GAS_MXP
    mxp.macro/S1NZNOMXPX
    mxp.macro/S2NZNOMXPX
  )
  ;; source selector
  (hub-into-mxp-trigger)
  ;; source columns
  (
    1
    hub.MXP_STAMP
    hub.CONTEXT_NUMBER
    hub.misc/MXP_INST
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
    hub.misc/MXP_MXPX
    hub.misc/MXP_GAS_MXP
    hub.misc/MXP_SIZE_1_NONZERO_NO_MXPX
    hub.misc/MXP_SIZE_2_NONZERO_NO_MXPX
  ))
