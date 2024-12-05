(module shf)

(defcolumns
  (SHIFT_STAMP                :i32)
  (IOMF                       :binary)
  (COUNTER                    :i8)
  (ONE_LINE_INSTRUCTION       :binary)
  (ARG_1_HI                   :i128)
  (ARG_1_LO                   :i128)
  (ARG_2_HI                   :i128)
  (ARG_2_LO                   :i128)
  (RES_HI                     :i128)
  (RES_LO                     :i128)
  (INST                       :byte :display :opcode)
  (SHIFT_DIRECTION            :binary@prove)
  (BITS                       :binary@prove)
  (NEG                        :binary@prove)
  (KNOWN                      :binary@prove)
  (LEFT_ALIGNED_SUFFIX_HIGH   :byte) ;decoded
  (RIGHT_ALIGNED_PREFIX_HIGH  :byte) ;decoded
  (LEFT_ALIGNED_SUFFIX_LOW    :byte) ;decoded
  (RIGHT_ALIGNED_PREFIX_LOW   :byte) ;decoded
  (ONES                       :byte) ;decoded
  (LOW_3                      :i128)
  (MICRO_SHIFT_PARAMETER      :i8)
  (BIT_1                      :binary@prove)
  (BIT_2                      :binary@prove)
  (BIT_3                      :binary@prove)
  (BIT_4                      :binary@prove)
  (BIT_B_3                    :binary@prove)
  (BIT_B_4                    :binary@prove)
  (BIT_B_5                    :binary@prove)
  (BIT_B_6                    :binary@prove)
  (BIT_B_7                    :binary@prove)
  (BYTE_1                     :byte@prove)
  (BYTE_2                     :byte@prove)
  (BYTE_3                     :byte@prove)
  (BYTE_4                     :byte@prove)
  (BYTE_5                     :byte@prove)
  (ACC_1                      :i128)
  (ACC_2                      :i128)
  (ACC_3                      :i128)
  (ACC_4                      :i128)
  (ACC_5                      :i128)
  (SHB_3_HI                   :byte)
  (SHB_3_LO                   :byte)
  (SHB_4_HI                   :byte)
  (SHB_4_LO                   :byte)
  (SHB_5_HI                   :byte)
  (SHB_5_LO                   :byte)
  (SHB_6_HI                   :byte)
  (SHB_6_LO                   :byte)
  (SHB_7_HI                   :byte)
  (SHB_7_LO                   :byte))

;; aliases
(defalias
  STAMP SHIFT_STAMP
  SHD   SHIFT_DIRECTION
  OLI   ONE_LINE_INSTRUCTION
  µSHP  MICRO_SHIFT_PARAMETER
  CT    COUNTER
  LA_HI LEFT_ALIGNED_SUFFIX_HIGH
  RA_HI RIGHT_ALIGNED_PREFIX_HIGH
  LA_LO LEFT_ALIGNED_SUFFIX_LOW
  RA_LO RIGHT_ALIGNED_PREFIX_LOW)


