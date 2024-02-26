(defpurefun (selector-bin-to-binRT)
  (+ bin.IS_AND bin.IS_OR bin.IS_XOR bin.IS_NOT))

(deflookup 
  bin-lookup-table-high
  ;reference columns
  (
    binRT.INST
    binRT.RESULT_BYTE
    binRT.INPUT_BYTE_1
    binRT.INPUT_BYTE_2
  )
  ;source columns 
  (
    (* bin.INST (selector-bin-to-binRT))
    (* bin.XXX_BYTE_HI (selector-bin-to-binRT))
    (* bin.BYTE_1 (selector-bin-to-binRT))
    (* bin.BYTE_3 (selector-bin-to-binRT))
  ))

(deflookup 
  bin-lookup-table-low
  ;reference columns
  (
    binRT.INST
    binRT.RESULT_BYTE
    binRT.INPUT_BYTE_1
    binRT.INPUT_BYTE_2
  )
  ;source columns 
  (
    (* bin.INST (selector-bin-to-binRT))
    (* bin.XXX_BYTE_LO (selector-bin-to-binRT))
    (* bin.BYTE_2 (selector-bin-to-binRT))
    (* bin.BYTE_4 (selector-bin-to-binRT))
  ))


