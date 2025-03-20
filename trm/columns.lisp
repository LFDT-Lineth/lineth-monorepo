(module trm)

(defcolumns
  (STAMP          :i24)
  (RAW_ADDRESS_HI :i128)
  (RAW_ADDRESS_LO :i128)
  (TRM_ADDRESS_HI :i32)
  (IS_PRECOMPILE  :binary@prove)
  (CT             :i4)
  (ACC_HI         :i128)
  (ACC_LO         :i128)
  (ACC_T          :i32)
  (PLATEAU_BIT    :binary@prove)
  (ONE            :binary@prove)
  (BYTE_HI        :byte@prove)
  (BYTE_LO        :byte@prove))

(defalias
  PBIT    PLATEAU_BIT)
