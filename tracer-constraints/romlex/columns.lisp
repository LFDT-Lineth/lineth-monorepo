(module romlex)

(defcolumns
  (CODE_FRAGMENT_INDEX :i32)
  (CODE_FRAGMENT_INDEX_INFTY :i32)
  (CODE_SIZE :i32)
  (ADDRESS_HI :i32)
  (ADDRESS_LO :i128)
  (DEPLOYMENT_NUMBER :i16)
  (DEPLOYMENT_STATUS :binary@prove)
  (CODE_HASH_HI :i128 :display :hex)
  (CODE_HASH_LO :i128 :display :hex))

(defalias
  CFI CODE_FRAGMENT_INDEX)


