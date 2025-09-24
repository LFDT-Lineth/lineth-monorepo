(module blsreftable)

(defcolumns
    (PRC_NAME   :byte)
    (NUM_INPUTS :i8) ;; greatest value is 128
    (DISCOUNT   :i10) ;; greatest value is 1000
)
