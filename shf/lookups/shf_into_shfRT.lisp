(deflookup 
  shf-into-shfRT-hi
  ;reference columns
  (
    shfRT.IOMF
    shfRT.BYTE1
    shfRT.MSHP
    shfRT.LAS
    shfRT.RAP
    shfRT.ONES
  )
  ;source columns 
  (
    shf.IOMF
    shf.BYTE_2
    shf.MICRO_SHIFT_PARAMETER
    shf.LEFT_ALIGNED_SUFFIX_HIGH
    shf.RIGHT_ALIGNED_PREFIX_HIGH
    shf.ONES
  ))

(deflookup 
  shf-into-shfRT-lo
  ;reference columns
  (
    shfRT.IOMF
    shfRT.BYTE1
    shfRT.MSHP
    shfRT.LAS
    shfRT.RAP
  )
  ;source columns 
  (
    shf.IOMF
    shf.BYTE_3
    shf.MICRO_SHIFT_PARAMETER
    shf.LEFT_ALIGNED_SUFFIX_LOW
    shf.RIGHT_ALIGNED_PREFIX_LOW
  ))


