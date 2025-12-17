(module oob)


(defun (lookup-sum k)                       (+    (shift   ADD_FLAG            k)
                                                  (shift   MOD_FLAG            k)
                                                  (shift   WCP_FLAG            k)
                                                  (shift   BLS_REF_TABLE_FLAG  k)
                                                  ))

(defun (wght-lookup-sum k)                  (+    (*  1  (shift   ADD_FLAG             k) )
                                                  (*  2  (shift   MOD_FLAG             k) )
                                                  (*  3  (shift   WCP_FLAG             k) )
                                                  (*  4  (shift   BLS_REF_TABLE_FLAG   k) )
                                                  ))

