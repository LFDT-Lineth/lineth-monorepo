(module mmio)

(definterleaved
  CN_ABC
  (CN_A CN_B CN_C))

(definterleaved
  INDEX_ABC
  (INDEX_A INDEX_B INDEX_C))

(definterleaved
  MMIO_STAMP_3
  (MMIO_STAMP MMIO_STAMP MMIO_STAMP))

(definterleaved
  VAL_ABC
  (VAL_A VAL_B VAL_C))

(definterleaved
  VAL_ABC_NEW
  (VAL_A_NEW VAL_B_NEW VAL_C_NEW))

(defpermutation
  (CN_ABC_SORTED
   INDEX_ABC_SORTED
   MMIO_STAMP_3_SORTED
   VAL_ABC_SORTED
   VAL_ABC_NEW_SORTED)

  ((+ CN_ABC)
   (+ INDEX_ABC)
   (+ MMIO_STAMP_3)
   VAL_ABC
   VAL_ABC_NEW)
  )

(defconstraint memory-consistency (:guard CN_ABC_SORTED)
  (begin (if-zero (will-remain-constant! CN_ABC_SORTED)
                  (if-zero (will-remain-constant! INDEX_ABC_SORTED)
                           (if-not-zero (will-remain-constant! MMIO_STAMP_3_SORTED)
                                        (will-eq! VAL_ABC_SORTED VAL_ABC_NEW_SORTED))))
         (if-not-zero (will-remain-constant! CN_ABC_SORTED)
                      (will-eq! VAL_ABC_SORTED 0))
         (if-not-zero (will-remain-constant! INDEX_ABC_SORTED)
                      (will-eq! VAL_ABC_SORTED 0))))


