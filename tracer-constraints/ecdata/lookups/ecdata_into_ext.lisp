(defun (ec_data-into-ext-activation-flag)
  ecdata.EXT_FLAG)

(defclookup
  ecdata-into-ext
  ;; target columns
  (
   ext.A
   ext.B
   ext.M
   ext.RES
   ext.INST
  )
  ;; source selector
  (ec_data-into-ext-activation-flag)
  ;; source columns
  (
    (:: ecdata.EXT_ARG1_HI ecdata.EXT_ARG1_LO )
    (:: ecdata.EXT_ARG2_HI ecdata.EXT_ARG2_LO )
    (:: ecdata.EXT_ARG3_HI ecdata.EXT_ARG3_LO )
    (:: ecdata.EXT_RES_HI  ecdata.EXT_RES_LO  )
    ecdata.EXT_INST
  ))


