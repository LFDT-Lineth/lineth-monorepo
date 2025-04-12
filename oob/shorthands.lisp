(module oob)


(defun (flag-sum-inst)                      (+    IS_JUMP IS_JUMPI
                                                  IS_RDC
                                                  IS_CDL
                                                  IS_XCALL
                                                  IS_CALL
                                                  ;; IS_XCREATE ;; XXXXXX
                                                  IS_CREATE
                                                  IS_SSTORE
                                                  IS_DEPLOYMENT))

(defun (flag-sum-prc-common)                (+    IS_ECRECOVER
                                                  IS_SHA2
                                                  IS_RIPEMD
                                                  IS_IDENTITY
                                                  IS_ECADD
                                                  IS_ECMUL
                                                  IS_ECPAIRING))

(defun (flag-sum-prc-blake)                 (+    IS_BLAKE2F_CDS
                                                  IS_BLAKE2F_PARAMS))

(defun (flag-sum-prc-modexp)                (+    IS_MODEXP_CDS
                                                  IS_MODEXP_XBS
                                                  IS_MODEXP_LEAD
                                                  IS_MODEXP_PRICING
                                                  IS_MODEXP_EXTRACT))

(defun (flag-sum-prc)                       (+    (flag-sum-prc-common)
                                                  (flag-sum-prc-blake)
                                                  (flag-sum-prc-modexp)))

(defun (flag-sum)                           (+    (flag-sum-inst)
                                                  (flag-sum-prc)))

(defun (wght-sum-inst)                      (+    (* OOB_INST_JUMP             IS_JUMP)
                                                  (* OOB_INST_JUMPI            IS_JUMPI)
                                                  (* OOB_INST_RDC              IS_RDC)
                                                  (* OOB_INST_CDL              IS_CDL)
                                                  (* OOB_INST_XCALL            IS_XCALL)
                                                  (* OOB_INST_CALL             IS_CALL)
                                                  ;; (* OOB_INST_XCREATE          IS_XCREATE) ;; XXXXXX
                                                  (* OOB_INST_CREATE           IS_CREATE)
                                                  (* OOB_INST_SSTORE           IS_SSTORE)
                                                  (* OOB_INST_DEPLOYMENT       IS_DEPLOYMENT)))

(defun (wght-sum-prc-common)                (+    (* OOB_INST_ECRECOVER        IS_ECRECOVER)
                                                  (* OOB_INST_SHA2             IS_SHA2)
                                                  (* OOB_INST_RIPEMD           IS_RIPEMD)
                                                  (* OOB_INST_IDENTITY         IS_IDENTITY)
                                                  (* OOB_INST_ECADD            IS_ECADD)
                                                  (* OOB_INST_ECMUL            IS_ECMUL)
                                                  (* OOB_INST_ECPAIRING        IS_ECPAIRING)))

(defun (wght-sum-prc-blake)                 (+    (* OOB_INST_BLAKE_CDS        IS_BLAKE2F_CDS)
                                                  (* OOB_INST_BLAKE_PARAMS     IS_BLAKE2F_PARAMS)))

(defun (wght-sum-prc-modexp)                (+    (* OOB_INST_MODEXP_CDS       IS_MODEXP_CDS)
                                                  (* OOB_INST_MODEXP_XBS       IS_MODEXP_XBS)
                                                  (* OOB_INST_MODEXP_LEAD      IS_MODEXP_LEAD)
                                                  (* OOB_INST_MODEXP_PRICING   IS_MODEXP_PRICING)
                                                  (* OOB_INST_MODEXP_EXTRACT   IS_MODEXP_EXTRACT)))

(defun (wght-sum-prc)                       (+    (wght-sum-prc-common)
                                                  (wght-sum-prc-blake)
                                                  (wght-sum-prc-modexp)))

(defun (wght-sum)                           (+    (wght-sum-inst)
                                                  (wght-sum-prc)))

(defun (maxct-sum-inst)                     (+    (* CT_MAX_JUMP               IS_JUMP)
                                                  (* CT_MAX_JUMPI              IS_JUMPI)
                                                  (* CT_MAX_RDC                IS_RDC)
                                                  (* CT_MAX_CDL                IS_CDL)
                                                  (* CT_MAX_XCALL              IS_XCALL)
                                                  (* CT_MAX_CALL               IS_CALL)
                                                  ;; (* CT_MAX_XCREATE            IS_XCREATE) ;; XXXXXX
                                                  (* CT_MAX_CREATE             IS_CREATE)
                                                  (* CT_MAX_SSTORE             IS_SSTORE)
                                                  (* CT_MAX_DEPLOYMENT         IS_DEPLOYMENT)))

(defun (maxct-sum-prc-common)               (+    (* CT_MAX_ECRECOVER          IS_ECRECOVER)
                                                  (* CT_MAX_SHA2               IS_SHA2)
                                                  (* CT_MAX_RIPEMD             IS_RIPEMD)
                                                  (* CT_MAX_IDENTITY           IS_IDENTITY)
                                                  (* CT_MAX_ECADD              IS_ECADD)
                                                  (* CT_MAX_ECMUL              IS_ECMUL)
                                                  (* CT_MAX_ECPAIRING          IS_ECPAIRING)))

(defun (maxct-sum-prc-blake)                (+    (* CT_MAX_BLAKE2F_CDS IS_BLAKE2F_CDS)
                                                  (* CT_MAX_BLAKE2F_PARAMS IS_BLAKE2F_PARAMS)))

(defun (maxct-sum-prc-modexp)               (+    (* CT_MAX_MODEXP_CDS IS_MODEXP_CDS)
                                                  (* CT_MAX_MODEXP_XBS IS_MODEXP_XBS)
                                                  (* CT_MAX_MODEXP_LEAD IS_MODEXP_LEAD)
                                                  (* CT_MAX_MODEXP_PRICING IS_MODEXP_PRICING)
                                                  (* CT_MAX_MODEXP_EXTRACT IS_MODEXP_EXTRACT)))

(defun (maxct-sum-prc)                      (+    (maxct-sum-prc-common)
                                                  (maxct-sum-prc-blake)
                                                  (maxct-sum-prc-modexp)))

(defun (maxct-sum)                          (+    (maxct-sum-inst)
                                                  (maxct-sum-prc)))

(defun (lookup-sum k)                       (+    (shift ADD_FLAG k)
                                                  (shift MOD_FLAG k)
                                                  (shift WCP_FLAG k)))

(defun (wght-lookup-sum k)                  (+    (* 1 (shift ADD_FLAG k))
                                                  (* 2 (shift MOD_FLAG k))
                                                  (* 3 (shift WCP_FLAG k))))

(defun (assumption---fresh-new-stamp)       (- STAMP (prev STAMP)))
