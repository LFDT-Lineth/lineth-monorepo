(module shakiradata)

(defcolumns 
  (RIPSHA_STAMP :i32)
  (ID :i32)
  (PHASE :byte)
  (INDEX :i32)
  (INDEX_MAX :i32)
  (LIMB :i128)
  (nBYTES :i8)
  (nBYTES_ACC :i32)
  (TOTAL_SIZE :i32)
  (IS_KECCAK_DATA :binary@prove)
  (IS_KECCAK_RESULT :binary@prove)
  (IS_SHA2_DATA :binary@prove)
  (IS_SHA2_RESULT :binary@prove)
  (IS_RIPEMD_DATA :binary@prove)
  (IS_RIPEMD_RESULT :binary@prove)
  (SELECTOR_KECCAK_RES_HI :binary)
  (SELECTOR_KECCAK_RES_LO :binary))


