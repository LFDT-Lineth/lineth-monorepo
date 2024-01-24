(module romLex)

(defcolumns 
  (CODE_FRAGMENT_INDEX :i32)
  (CODE_FRAGMENT_INDEX_INFTY :i32)
  (CODE_SIZE :i32)
  (ADDR_HI :i32)
  (ADDR_LO :i128)
  (DEP_NUMBER :i16)
  (DEP_STATUS :binary@prove)
  (COMMIT_TO_STATE :binary@prove)
  (READ_FROM_STATE :binary@prove))

(defalias 
  CFI CODE_FRAGMENT_INDEX)


