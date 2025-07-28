(defun (selector-bin-to-binreftable)
  (force-bin (+ bin.IS_AND bin.IS_OR bin.IS_XOR bin.IS_NOT)))

(defclookup
  bin-into-binreftable-high
  ;reference columns
  (
    binreftable.INST
    binreftable.RESULT_BYTE
    binreftable.INPUT_BYTE_1
    binreftable.INPUT_BYTE_2
  )
  ;source selector
  (selector-bin-to-binreftable)
  ;source columns
  (
    bin.INST
    bin.XXX_BYTE_HI
    bin.BYTE_1
    bin.BYTE_3
  ))

(defclookup
  bin-into-binreftable-low
  ;reference columns
  (
    binreftable.INST
    binreftable.RESULT_BYTE
    binreftable.INPUT_BYTE_1
    binreftable.INPUT_BYTE_2
    )
  ;source selector
  (selector-bin-to-binreftable)
  ;source columns
  (
    bin.INST
    bin.XXX_BYTE_LO
    bin.BYTE_2
    bin.BYTE_4
  ))


