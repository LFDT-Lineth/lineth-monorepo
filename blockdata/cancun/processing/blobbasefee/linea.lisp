(module blockdata)

(defconst (BLOB_BASE_FEE_ENABLE :binary :extern) 1)

(defconstraint   blobbasefee-value
                 (:guard (* (blobbasefee-precondition) BLOB_BASE_FEE_ENABLE))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin (eq!  (curr-BLOBBASEFEE-hi)  0)
                        (eq!  (curr-BLOBBASEFEE-lo)  LINEA_BLOB_BASE_FEE)))
