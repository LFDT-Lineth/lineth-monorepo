(module oob)


(defun (ct-max-sum-inst)                    (+    (*   CT_MAX_JUMP               IS_JUMP       )
                                                  (*   CT_MAX_JUMPI              IS_JUMPI      )
                                                  (*   CT_MAX_RDC                IS_RDC        )
                                                  (*   CT_MAX_CDL                IS_CDL        )
                                                  (*   CT_MAX_XCALL              IS_XCALL      )
                                                  (*   CT_MAX_CALL               IS_CALL       )
                                                  (*   CT_MAX_XCREATE            IS_XCREATE    )
                                                  (*   CT_MAX_CREATE             IS_CREATE     )
                                                  (*   CT_MAX_SSTORE             IS_SSTORE     )
                                                  (*   CT_MAX_DEPLOYMENT         IS_DEPLOYMENT )
                                                  ))

(defun (ct-max-sum-prc-common)              (+    (*   CT_MAX_ECRECOVER          IS_ECRECOVER )
                                                  (*   CT_MAX_SHA2               IS_SHA2      )
                                                  (*   CT_MAX_RIPEMD             IS_RIPEMD    )
                                                  (*   CT_MAX_IDENTITY           IS_IDENTITY  )
                                                  (*   CT_MAX_ECADD              IS_ECADD     )
                                                  (*   CT_MAX_ECMUL              IS_ECMUL     )
                                                  (*   CT_MAX_ECPAIRING          IS_ECPAIRING )
                                                  (ct-max-sum-prc-bls)
                                                  (ct-max-sum-prc-osaka-precompiles)
                                                  ))

(defun (ct-max-sum-prc-blake)               (+    (*   CT_MAX_BLAKE2F_CDS         IS_BLAKE2F_CDS    )
                                                  (*   CT_MAX_BLAKE2F_PARAMS      IS_BLAKE2F_PARAMS )
                                                  ))

(defun (ct-max-sum-prc-modexp)              (+    (*   CT_MAX_MODEXP_CDS          IS_MODEXP_CDS     )
                                                  (*   CT_MAX_MODEXP_XBS          IS_MODEXP_XBS     )
                                                  (*   CT_MAX_MODEXP_LEAD         IS_MODEXP_LEAD    )
                                                  (*   CT_MAX_MODEXP_PRICING      IS_MODEXP_PRICING )
                                                  (*   CT_MAX_MODEXP_EXTRACT      IS_MODEXP_EXTRACT )
                                                  ))

(defun (ct-max-sum-prc-cancun-precompiles)  (+    (*   CT_MAX_POINT_EVALUATION    IS_POINT_EVALUATION )))

(defun (ct-max-sum-prc-prague-precompiles)  (+    (*   CT_MAX_BLS_G1_ADD          IS_BLS_G1_ADD        )
                                                  (*   CT_MAX_BLS_G1_MSM          IS_BLS_G1_MSM        )
                                                  (*   CT_MAX_BLS_G2_ADD          IS_BLS_G2_ADD        )
                                                  (*   CT_MAX_BLS_G2_MSM          IS_BLS_G2_MSM        )
                                                  (*   CT_MAX_BLS_PAIRING_CHECK   IS_BLS_PAIRING_CHECK )
                                                  (*   CT_MAX_BLS_MAP_FP_TO_G1    IS_BLS_MAP_FP_TO_G1  )
                                                  (*   CT_MAX_BLS_MAP_FP2_TO_G2   IS_BLS_MAP_FP2_TO_G2 )
                                                  ))

(defun (ct-max-sum-prc-osaka-precompiles)         (*   CT_MAX_P256_VERIFY         IS_P256_VERIFY ) )

(defun (ct-max-sum-prc-bls)                 (+    (ct-max-sum-prc-cancun-precompiles)
                                                  (ct-max-sum-prc-prague-precompiles)
                                                  ))

(defun (ct-max-sum-prc)                     (+    (ct-max-sum-prc-common)
                                                  (ct-max-sum-prc-blake)
                                                  (ct-max-sum-prc-modexp)
                                                  ))

(defun (ct-max-sum)                         (+    (ct-max-sum-inst)
                                                  (ct-max-sum-prc)
                                                  ))

