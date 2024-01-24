(module trm)

(defcolumns 
  (STAMP :i24)
  (ADDR_HI :i128)
  (ADDR_LO :i128)
  (TRM_ADDR_HI :i4)
  (IS_PREC :binary@prove)
  ;;
  (CT :byte)
  (ACC_HI :i128)
  (ACC_LO :i128)
  (ACC_T :i128)
  (PBIT :binary@prove)
  (ONE :binary@prove)
  (BYTE_HI :byte@prove)
  (BYTE_LO :byte@prove))


