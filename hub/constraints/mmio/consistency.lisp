(module mmio)

(defcolumns
  (CN_ABC :INTERLEAVED (CN_A CN_B CN_C))
  (INDEX_ABC :INTERLEAVED (INDEX_A INDEX_B INDEX_C))
  (MICRO_STAMP_3 :INTERLEAVED (MICRO_INSTRUCTION_STAMP MICRO_INSTRUCTION_STAMP MICRO_INSTRUCTION_STAMP))
  (VAL_ABC :INTERLEAVED (VAL_A VAL_B VAL_C))
  (VAL_ABC_NEW :INTERLEAVED (VAL_A_NEW VAL_B_NEW VAL_C_NEW))
)


(defpermutation
  (CN_ABC_SORTED INDEX_ABC_SORTED MICRO_STAMP_3_SORTED VAL_ABC_SORTED VAL_ABC_NEW_SORTED)
  (CN_ABC INDEX_ABC MICRO_STAMP_3 VAL_ABC VAL_ABC_NEW)
)


(defconstraint memory-consistency ()
  (begin
    (if-not-zero CN_ABC_SORTED
      (if-zero (remains-constant CN_ABC_SORTED)
        (if-zero (remains-constant INDEX_ABC_SORTED)
          (if-not-zero (remains-constant MICRO_STAMP_3_SORTED)
            (will-eq VAL_ABC_SORTED VAL_ABC_NEW_SORTED)
          )
        )
      )
    )

    (if-not-zero (remains-constant CN_ABC_SORTED)
     (will-eq VAL_ABC_SORTED 0)
    )

    (if-not-zero (remains-constant INDEX_ABC_SORTED)
      (will-eq VAL_ABC_SORTED 0)
    )
  )
)
