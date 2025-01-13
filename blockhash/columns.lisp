(module blockhash)

(defcolumns
  (IOMF              :binary@prove)
  (MACRO             :binary@prove)
  (PRPRC             :binary@prove)
  (CT                :i8)
  (CT_MAX            :i8))

(defperspective macro
                ;; selector
                MACRO
                ;; macro-instruction fields
                (
                 (REL_BLOCK         :i16)
                 (ABS_BLOCK         :i48)
                 (BLOCKHASH_VAL_HI  :i128)
                 (BLOCKHASH_VAL_LO  :i128)
                 (BLOCKHASH_ARG_HI  :i128)
                 (BLOCKHASH_ARG_LO  :i128)
                 (BLOCKHASH_RES_HI  :i128)
                 (BLOCKHASH_RES_LO  :i128)
                 )
                )

(defperspective    preprocessing
                   ;; selector
                   PRPRC
                   ;; instruction pre-processing fields
                   (
                    (EXO_ARG_1_HI      :i128)
                    (EXO_ARG_1_LO      :i128)
                    (EXO_ARG_2_HI      :i128)
                    (EXO_ARG_2_LO      :i128)
                    (EXO_INST          :i8)
                    (EXO_RES           :binary@prove)
                    )
                   )

