(module oob)


(defun (wght-sum-inst)                      (+    (*   OOB_INST_JUMP             IS_JUMP       )
                                                  (*   OOB_INST_JUMPI            IS_JUMPI      )
                                                  (*   OOB_INST_RDC              IS_RDC        )
                                                  (*   OOB_INST_CDL              IS_CDL        )
                                                  (*   OOB_INST_XCALL            IS_XCALL      )
                                                  (*   OOB_INST_CALL             IS_CALL       )
                                                  (*   OOB_INST_XCREATE          IS_XCREATE    )
                                                  (*   OOB_INST_CREATE           IS_CREATE     )
                                                  (*   OOB_INST_SSTORE           IS_SSTORE     )
                                                  (*   OOB_INST_DEPLOYMENT       IS_DEPLOYMENT )
                                                  ))

(defun (wght-sum-prc-common)                (+    (*   OOB_INST_ECRECOVER        IS_ECRECOVER )
                                                  (*   OOB_INST_SHA2             IS_SHA2      )
                                                  (*   OOB_INST_RIPEMD           IS_RIPEMD    )
                                                  (*   OOB_INST_IDENTITY         IS_IDENTITY  )
                                                  (*   OOB_INST_ECADD            IS_ECADD     )
                                                  (*   OOB_INST_ECMUL            IS_ECMUL     )
                                                  (*   OOB_INST_ECPAIRING        IS_ECPAIRING )
                                                  (wght-sum-prc-bls)
                                                  (wght-sum-prc-osaka-precompiles)
                                                  ))

(defun (wght-sum-prc-blake)                 (+    (*   OOB_INST_BLAKE_CDS            IS_BLAKE2F_CDS    )
                                                  (*   OOB_INST_BLAKE_PARAMS         IS_BLAKE2F_PARAMS )
                                                  ))

(defun (wght-sum-prc-modexp)                (+    (*   OOB_INST_MODEXP_CDS           IS_MODEXP_CDS     )
                                                  (*   OOB_INST_MODEXP_XBS           IS_MODEXP_XBS     )
                                                  (*   OOB_INST_MODEXP_LEAD          IS_MODEXP_LEAD    )
                                                  (*   OOB_INST_MODEXP_PRICING       IS_MODEXP_PRICING )
                                                  (*   OOB_INST_MODEXP_EXTRACT       IS_MODEXP_EXTRACT )
                                                  ))

(defun (wght-sum-prc-cancun-precompiles)    (+    (*   OOB_INST_POINT_EVALUATION     IS_POINT_EVALUATION ) ))

(defun (wght-sum-prc-prague-precompiles)    (+    (*   OOB_INST_BLS_G1_ADD           IS_BLS_G1_ADD        )
                                                  (*   OOB_INST_BLS_G1_MSM           IS_BLS_G1_MSM        )
                                                  (*   OOB_INST_BLS_G2_ADD           IS_BLS_G2_ADD        )
                                                  (*   OOB_INST_BLS_G2_MSM           IS_BLS_G2_MSM        )
                                                  (*   OOB_INST_BLS_PAIRING_CHECK    IS_BLS_PAIRING_CHECK )
                                                  (*   OOB_INST_BLS_MAP_FP_TO_G1     IS_BLS_MAP_FP_TO_G1  )
                                                  (*   OOB_INST_BLS_MAP_FP2_TO_G2    IS_BLS_MAP_FP2_TO_G2 )
                                                  ))

(defun (wght-sum-prc-osaka-precompiles)     (+    (* OOB_INST_P256_VERIFY            IS_P256_VERIFY)))

(defun (wght-sum-prc-bls)                   (+    (wght-sum-prc-cancun-precompiles)
                                                  (wght-sum-prc-prague-precompiles)
                                                  ))

(defun (wght-sum-prc)                       (+    (wght-sum-prc-common)
                                                  (wght-sum-prc-blake)
                                                  (wght-sum-prc-modexp)
                                                  ))

(defun (wght-sum)                           (+    (wght-sum-inst)
                                                  (wght-sum-prc)
                                                  ))

