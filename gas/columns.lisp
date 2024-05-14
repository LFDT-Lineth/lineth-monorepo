(module gas)

(defcolumns 
  (STAMP :i32)
  (CT :i3)
  (GAS_ACTL :i32)
  (GAS_COST :i64)
  (OOGX :binary@prove)
  (BYTE :byte@prove :array [2])
  (ACC :i64 :array [2]))


