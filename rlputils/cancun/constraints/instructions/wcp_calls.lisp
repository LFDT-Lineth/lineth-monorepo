(module rlputils)

(defun (wcp-call-iszero  offset int-hi int-lo)
    (begin
    (eq! (shift compt/INST     offset)   EVM_INST_ISZERO   )
    (eq! (shift compt/ARG_1_HI offset)   int-hi            )
    (eq! (shift compt/ARG_1_LO offset)   int-lo            )))

(defun (wcp-call-gt      offset int1-hi int1-lo int2-lo)
    (begin
    (eq! (shift compt/INST     offset)   EVM_INST_GT   )
    (eq! (shift compt/ARG_1_HI offset)   int1-hi       )
    (eq! (shift compt/ARG_1_LO offset)   int1-lo       )
    (eq! (shift compt/ARG_2_LO offset)   int2-lo       )))

(defun (wcp-call-lt      offset int1-hi int1-lo int2-lo)
    (begin
    (eq! (shift compt/INST     offset)   EVM_INST_LT   )
    (eq! (shift compt/ARG_1_HI offset)   int1-hi       )
    (eq! (shift compt/ARG_1_LO offset)   int1-lo       )
    (eq! (shift compt/ARG_2_LO offset)   int2-lo       )))

(defun (wcp-call-eq      offset         int1-lo int2-lo)
    (begin
    (eq! (shift compt/INST     offset)   EVM_INST_EQ   )
    (eq! (shift compt/ARG_1_HI offset)   0             )
    (eq! (shift compt/ARG_1_LO offset)   int1-lo       )
    (eq! (shift compt/ARG_2_LO offset)   int2-lo       )))

(defun (wcp-call-geq     offset int1-hi int1-lo int2-lo)
    (begin
    (eq! (shift compt/INST     offset)   WCP_INST_GEQ  )
    (eq! (shift compt/ARG_1_HI offset)   int1-hi       )
    (eq! (shift compt/ARG_1_LO offset)   int1-lo       )
    (eq! (shift compt/ARG_2_LO offset)   int2-lo       )))

(defun (wcp-call-leq     offset int1-hi int1-lo int2-lo)
    (begin
    (eq! (shift compt/INST     offset)   WCP_INST_LEQ  )
    (eq! (shift compt/ARG_1_HI offset)   int1-hi       )
    (eq! (shift compt/ARG_1_LO offset)   int1-lo       )
    (eq! (shift compt/ARG_2_LO offset)   int2-lo       )))

(defun (result-must-be-true offset) (eq! (shift compt/RES offset) 1))