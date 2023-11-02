(module trm)

(defcolumns
    STAMP
    ADDR_HI
    ADDR_LO
    TRM_ADDR_HI
    (IS_PREC :binary)
    ;;
    CT
    (BYTE_HI :byte)
    (BYTE_LO :byte)
    ACC_HI
    ACC_LO
    ACC_T
    ;;
    (PBIT :binary)
    (ONES :binary)
)
