(defun (selector-bin-to-binreftable)
  (+ bin.IS_AND bin.IS_OR bin.IS_XOR bin.IS_NOT))

(deflookup
  bin-into-binreftable-high
  ;reference columns
  (
    binreftable.INST
    binreftable.RESULT_BYTE
    binreftable.INPUT_BYTE_1
    binreftable.INPUT_BYTE_2
  )
  ;source columns
  (
    (* bin.INST (selector-bin-to-binreftable))
    (* bin.XXX_BYTE_HI (selector-bin-to-binreftable))
    (* bin.BYTE_1 (selector-bin-to-binreftable))
    (* bin.BYTE_3 (selector-bin-to-binreftable))
  ))

(deflookup
  bin-into-binreftable-low
  ;reference columns
  (
    binreftable.INST
    binreftable.RESULT_BYTE
    binreftable.INPUT_BYTE_1
    binreftable.INPUT_BYTE_2
  )
  ;source columns
  (
    (* bin.INST (selector-bin-to-binreftable))
    (* bin.XXX_BYTE_LO (selector-bin-to-binreftable))
    (* bin.BYTE_2 (selector-bin-to-binreftable))
    (* bin.BYTE_4 (selector-bin-to-binreftable))
  ))


