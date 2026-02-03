(module blake2fmodexpdata)

(defcolumns
  (STAMP                :i16)
  (ID                   :i32)
  (PHASE                :byte)
  (INDEX                :byte :display :dec)
  (INDEX_MAX            :byte :display :dec)
  (LIMB                 :i128 :display :bytes)
  (IS_MODEXP_BASE       :binary@prove)
  (IS_MODEXP_EXPONENT   :binary@prove)
  (IS_MODEXP_MODULUS    :binary@prove)
  (IS_MODEXP_RESULT     :binary@prove)
  (IS_BLAKE_DATA        :binary@prove)
  (IS_BLAKE_PARAMS      :binary@prove)
  (IS_BLAKE_RESULT      :binary@prove)
  )

(defun (blake2f-selector)
  (== 1 ( * (- 1 (prev blake2fmodexpdata.IS_BLAKE_DATA)) blake2fmodexpdata.IS_BLAKE_DATA)))

(defcall
 (
  (shift LIMB 15) (shift LIMB 16) (shift LIMB 17) (shift LIMB 18)
 )
  blake2f
 (
  (shift LIMB 0)  (shift LIMB 1)  (shift LIMB 2)  (shift LIMB 3) (shift LIMB 4)
  (shift LIMB 5)  (shift LIMB 6)  (shift LIMB 7)  (shift LIMB 8) (shift LIMB 9)
  (shift LIMB 10) (shift LIMB 11) (shift LIMB 12) (i64 (shift LIMB 13)) (i1 (shift LIMB 14))
  )
  (blake2f-selector)
 )
