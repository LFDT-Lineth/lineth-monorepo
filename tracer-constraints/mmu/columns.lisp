(module mmu)

(defcolumns
  ;; shared columns
  (STAMP      :i32 :display :dec)
  (MMIO_STAMP :i32 :display :dec)
  ;; perspective selector
  (MACRO :binary@prove)
  (PRPRC :binary@prove)
  (MICRO :binary@prove)
  ;; OUTPUT OF THE PREPROCESSING
  (TOT :i32 :display :dec)
  (TOTLZ :i32 :display :dec)
  (TOTNT :i32 :display :dec)
  (TOTRZ :i32 :display :dec)
  (OUT :i64 :array [5])
  (BIN :binary :array [5])
  ;; MMU INSTRUCTION FLAG
  (IS_MLOAD                                :binary@prove)
  (IS_MSTORE                               :binary@prove)
  (IS_MSTORE8                              :binary@prove)
  (IS_INVALID_CODE_PREFIX                  :binary@prove)
  (IS_RIGHT_PADDED_WORD_EXTRACTION         :binary@prove)
  (IS_RAM_TO_EXO_WITH_PADDING              :binary@prove)
  (IS_EXO_TO_RAM_TRANSPLANTS               :binary@prove)
  (IS_RAM_TO_RAM_SANS_PADDING              :binary@prove)
  (IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA    :binary@prove)
  (IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING :binary@prove)
  (IS_MODEXP_ZERO                          :binary@prove)
  (IS_MODEXP_DATA                          :binary@prove)
  (IS_BLAKE                                :binary@prove)
  ;; USED ONLY IN MICRO ROW BUT ARE SHARED
  (LZRO     :binary@prove)
  (NT_ONLY  :binary@prove)
  (NT_FIRST :binary@prove)
  (NT_MDDL  :binary@prove)
  (NT_LAST  :binary@prove)
  (RZ_ONLY  :binary@prove)
  (RZ_FIRST :binary@prove)
  (RZ_MDDL  :binary@prove)
  (RZ_LAST  :binary@prove))

(defperspective macro

  ;; selector
  MACRO
  ((INST :i16 :display :hex)
   (SRC_ID :i64)
   (TGT_ID :i64)
   (AUX_ID :i64)
   (SRC_OFFSET_HI :i128)
   (SRC_OFFSET_LO :i128)
   (TGT_OFFSET_LO :i64)
   (SIZE          :i64)
   (REF_OFFSET    :i64)
   (REF_SIZE      :i64)
   (SUCCESS_BIT   :binary)
   (LIMB_1        :i128)
   (LIMB_2        :i128)
   (PHASE         :i32)
   (EXO_SUM       :i32)))

(defperspective prprc

  ;; selector
  PRPRC
  ((CT           :i16)
   (EUC_FLAG     :binary)
   (EUC_A        :i64)
   (EUC_B        :i64)
   (EUC_QUOT     :i64)
   (EUC_REM      :i64)
   (EUC_CEIL     :i64)
   (WCP_FLAG     :binary)
   (WCP_ARG_1_HI :i128)
   (WCP_ARG_1_LO :i128)
   (WCP_ARG_2_LO :i128)
   (WCP_RES      :binary)
   (WCP_INST     :byte :display :opcode)))

(defperspective micro

  ;; selector
  MICRO
  ((INST :i16 :display :hex)
   (SIZE :byte :display :dec)
   (SLO :i64 :display :dec)
   (SBO :byte :display :dec)
   (TLO :i64 :display :dec)
   (TBO :byte :display :dec)
   (LIMB :i128)
   (CN_S :i64 :display :dec)
   (CN_T :i64 :display :dec)
   (SUCCESS_BIT :binary)
   (EXO_SUM :i32)
   (PHASE :i32)
   (EXO_ID :i32)
   (KEC_ID :i32)
   (TOTAL_SIZE :i64 :display :dec)))


