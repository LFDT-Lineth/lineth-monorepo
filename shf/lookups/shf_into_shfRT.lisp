(deflookup
  shf-into-shfreftable-hi
  ;reference columns
  (
    shfreftable.IOMF
    shfreftable.BYTE1
    shfreftable.MSHP
    shfreftable.LAS
    shfreftable.RAP
    shfreftable.ONES
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
  shf-into-shfreftable-lo
  ;reference columns
  (
    shfreftable.IOMF
    shfreftable.BYTE1
    shfreftable.MSHP
    shfreftable.LAS
    shfreftable.RAP
  )
  ;source columns
  (
    shf.IOMF
    shf.BYTE_3
    shf.MICRO_SHIFT_PARAMETER
    shf.LEFT_ALIGNED_SUFFIX_LOW
    shf.RIGHT_ALIGNED_PREFIX_LOW
  ))


