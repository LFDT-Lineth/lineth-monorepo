(defpurefun (selector-bin-to-binRT)
  (+ bin.IS_AND bin.IS_OR bin.IS_XOR bin.IS_NOT))

(deflookup 
  bin-lookup-table-high
  ;reference columns
  (
    binRT.IOMF
    binRT.INST
    binRT.RESULT_BYTE
    binRT.INPUT_BYTE_1
    binRT.INPUT_BYTE_2
  )
  ;source columns 
  (
    (selector-bin-to-binRT)
    bin.INST
    bin.XXX_BYTE_HI
    bin.BYTE_1
    bin.BYTE_3
  ))

(deflookup 
  bin-lookup-table-low
  ;reference columns
  (
    binRT.IOMF
    binRT.INST
    binRT.RESULT_BYTE
    binRT.INPUT_BYTE_1
    binRT.INPUT_BYTE_2
  )
  ;source columns 
  (
    (selector-bin-to-binRT)
    bin.INST
    bin.XXX_BYTE_LO
    bin.BYTE_2
    bin.BYTE_4
  ))


