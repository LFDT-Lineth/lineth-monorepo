(module oob)


(defun (flag-sum-inst)                         (+    IS_JUMP IS_JUMPI
                                                     IS_RDC
                                                     IS_CDL
                                                     IS_XCALL
                                                     IS_CALL
                                                     IS_XCREATE
                                                     IS_CREATE
                                                     IS_SSTORE
                                                     IS_DEPLOYMENT
                                                     ))

(defun (flag-sum-prc-common)                   (+    (flag-sum-london-common-precompiles)
                                                     (flag-sum-cancun-precompiles)
                                                     (flag-sum-prague-precompiles)
                                                     (flag-sum-osaka-precompiles)
                                                     ))

(defun (flag-sum-london-common-precompiles)    (+    IS_ECRECOVER
                                                     IS_SHA2
                                                     IS_RIPEMD
                                                     IS_IDENTITY
                                                     IS_ECADD
                                                     IS_ECMUL
                                                     IS_ECPAIRING
                                                     ))

(defun (flag-sum-cancun-precompiles)           (+    IS_POINT_EVALUATION ))

(defun (flag-sum-prague-precompiles)                (+  (flag-sum-prague-precompiles-fixed-size)
                                                        (flag-sum-prague-precompiles-variable-size)
                                                        ))

(defun (flag-sum-prague-precompiles-fixed-size)     (+  IS_BLS_G1_ADD
                                                        IS_BLS_G2_ADD
                                                        IS_BLS_MAP_FP_TO_G1
                                                        IS_BLS_MAP_FP2_TO_G2
                                                        ))

(defun (flag-sum-prague-precompiles-variable-size)  (+  IS_BLS_G1_MSM
                                                        IS_BLS_G2_MSM
                                                        IS_BLS_PAIRING_CHECK
                                                        ))

(defun (flag-sum-osaka-precompiles)         (+    IS_P256_VERIFY ))

(defun (flag-sum-prc-blake)                 (+    IS_BLAKE2F_CDS
                                                  IS_BLAKE2F_PARAMS))

(defun (flag-sum-prc-modexp)                (+    IS_MODEXP_CDS
                                                  IS_MODEXP_XBS
                                                  IS_MODEXP_LEAD
                                                  IS_MODEXP_PRICING
                                                  IS_MODEXP_EXTRACT
                                                  ))

(defun (flag-sum-prc)                       (+    (flag-sum-prc-common)
                                                  (flag-sum-prc-blake)
                                                  (flag-sum-prc-modexp)
                                                  ))

(defun (flag-sum)                           (+    (flag-sum-inst)
                                                  (flag-sum-prc)
                                                  ))

