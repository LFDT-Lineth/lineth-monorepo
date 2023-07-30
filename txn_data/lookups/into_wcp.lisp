(defplookup txn_data_into_wcp
    ; target columns
    (
        wcp.ARGUMENT_1_HI
        wcp.ARGUMENT_1_LO
        wcp.ARGUMENT_2_HI
        wcp.ARGUMENT_2_LO
        wcp.INST
        wcp.RESULT_LO
    )
    ; source columns
    (
        txn_data.ZEROCOL
        txn_data.WCP_ARG_ONE_LO
        txn_data.ZEROCOL
        txn_data.WCP_ARG_TWO_LO
        txn_data.WCP_INST
        txn_data.WCP_RES_LO
    )
)
