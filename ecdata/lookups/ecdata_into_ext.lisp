(defun (ec_data-into-ext-activation-flag)
  ecdata.EXT_FLAG)

(deflookup
  ecdata-into-ext
  ; target columns
  (
    ext.ARG_1_HI
    ext.ARG_1_LO
    ext.ARG_2_HI
    ext.ARG_2_LO
    ext.ARG_3_HI
    ext.ARG_3_LO
    ext.RES_HI
    ext.RES_LO
    ext.INST
  )
  ; source columns
  (
    (* ecdata.EXT_ARG1_HI (ec_data-into-ext-activation-flag))
    (* ecdata.EXT_ARG1_LO (ec_data-into-ext-activation-flag))
    (* ecdata.EXT_ARG2_HI (ec_data-into-ext-activation-flag))
    (* ecdata.EXT_ARG2_LO (ec_data-into-ext-activation-flag))
    (* ecdata.EXT_ARG3_HI (ec_data-into-ext-activation-flag))
    (* ecdata.EXT_ARG3_LO (ec_data-into-ext-activation-flag))
    (* ecdata.EXT_RES_HI (ec_data-into-ext-activation-flag))
    (* ecdata.EXT_RES_LO (ec_data-into-ext-activation-flag))
    (* ecdata.EXT_INST (ec_data-into-ext-activation-flag))
  ))


