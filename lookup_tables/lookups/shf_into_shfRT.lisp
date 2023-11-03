(deflookup shf-into-shfRT-hi
    ;reference columns
    (
        shfRT.BYTE
        shfRT.MSHP
        shfRT.LAS
        shfRT.RAP
        shfRT.ONES
        shfRT.IS_IN_RT
    )
    ;source columns 
    (
        shf.BYTE_2
        shf.MICRO_SHIFT_PARAMETER
        shf.LEFT_ALIGNED_SUFFIX_HIGH
        shf.RIGHT_ALIGNED_PREFIX_HIGH
        shf.ONES
        shf.IS_DATA
    )
)

(deflookup shf-into-shfRT-lo
    ;reference columns
    (       
        shfRT.BYTE
        shfRT.MSHP
        shfRT.LAS
        shfRT.RAP
        shfRT.IS_IN_RT
    )
    ;source columns 
    (
        shf.BYTE_3
        shf.MICRO_SHIFT_PARAMETER
        shf.LEFT_ALIGNED_SUFFIX_LOW
        shf.RIGHT_ALIGNED_PREFIX_LOW
        shf.IS_DATA
    )
)