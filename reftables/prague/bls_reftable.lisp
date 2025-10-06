(module blsreftable)

(defcolumns
    (PRC_NAME   :i16)
    (NUM_INPUTS :i8) ;; greatest value is 128
    (DISCOUNT   :i10) ;; greatest value is 1000
)
