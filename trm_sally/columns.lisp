(module trm)

(defcolumns
  STAMP
  ADDR_HI
  ADDR_LO
  TRM_ADDR_HI
  (IS_PREC :BOOLEAN)
  ;;
  CT
  (BYTE_HI :BYTE)
  (BYTE_LO :BYTE)
  ACC_HI
  ACC_LO
  ACC_T
  ;;
  (PBIT :BOOLEAN)
  (ONES :BOOLEAN)
  )
