(module rlputils)

(defun (flag-sum) 
        (force-bin 
        (+ 
            IS_INTEGER           
            IS_BYTE_STRING_PREFIX
            IS_BYTE32            
            IS_DATA_PRICING ))) 

(defun (wght-flag-sum) 
        (+ 
              (* RLP_UTILS_INST_INTEGER                    IS_INTEGER           )
              (* RLP_UTILS_INST_BYTE_STRING_PREFIX         IS_BYTE_STRING_PREFIX)
              (* RLP_UTILS_INST_BYTES32                    IS_BYTE32            )
              (* RLP_UTILS_INST_DATA_PRICING               IS_DATA_PRICING      )))

(defcomputedcolumn (IOMF :binary@prove) (flag-sum))

(defconstraint inst-decoding-macro-row (:guard MACRO)
    (eq! (wght-flag-sum) macro/INST))

(defconstraint inst-decoding-compt-row (:guard COMPT)
    (eq! (wght-flag-sum) (prev (wght-flag-sum))))