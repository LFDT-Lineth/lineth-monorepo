(module rlputils)

(defcolumns 
;; shared columns
(IOMF                    :binary@prove)
(MACRO                   :binary@prove)
(COMPT                   :binary@prove)
(CT                      :i8)
(CT_MAX                  :i8)
(ZERO_COUNTER            :i32)
(NONZ_COUNTER            :i32)
(IS_INTEGER              :binary@prove)
(IS_BYTE_STRING_PREFIX   :binary@prove)
(IS_BYTE32               :binary@prove)
(IS_DATA_PRICING         :binary@prove)
)

(defperspective macro
;; selector
MACRO
(
(INST                   :i8)
(DATA_1                 :i128)
(DATA_2                 :i128)
(DATA_3                 :binary)
(DATA_4                 :binary)
(DATA_5                 :binary)
(DATA_6                 :i128)
(DATA_7                 :i128)
(DATA_8                 :i8)
))

(defperspective compt
;; selector
COMPT
(
(ARG_1_HI               :i128)
(ARG_1_LO               :i128)
(ARG_2_LO               :i128)
(RES                    :binary)
(INST                   :byte)
(WCP_CT_MAX             :i8)
(SHF_FLAG               :binary)
(SHF_ARG                :i8)
(SHF_POWER              :i128)
(ACC                    :i128)
(LIMB                   :i128)
))