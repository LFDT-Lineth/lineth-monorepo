(module hub_v2)

(defcolumns
    ABSOLUTE_TRANSACTION_NUMBER
    BATCH_NUMBER
    ( TX_SKIP                                    :binary@prove )
    ( TX_WARM                                    :binary@prove )
    ( TX_INIT                                    :binary@prove )
    ( TX_EXEC                                    :binary@prove )
    ( TX_FINL                                    :binary@prove )
    HUB_STAMP
    HUB_STAMP_TRANSACTION_END                                     ;; for SELFDESTRUCT
    ( TRANSACTION_REVERTS                       :binary@prove )   ;; TODO obsolete ?
    ( CONTEXT_MAY_CHANGE                        :binary@prove )
    ( EXCEPTION_AHOY                            :binary@prove )

    ;; extra stamps
    HASH_INFO_STAMP
    LOG_INFO_STAMP
    MMU_STAMP
    MXP_STAMP

    ;; stamps for undoing operations
    DOM_STAMP
    SUB_STAMP

    ;; context data
    CONTEXT_NUMBER
    CONTEXT_NUMBER_NEW
    CALLER_CONTEXT_NUMBER

    ;;
    ( CONTEXT_WILL_REVERT                       :binary@prove )
    ( CONTEXT_GETS_REVERTED                     :binary@prove )
    ( CONTEXT_SELF_REVERTS                      :binary@prove )
    CONTEXT_REVERT_STAMP

    ;;
    CODE_FRAGMENT_INDEX
    PROGRAM_COUNTER
    PROGRAM_COUNTER_NEW
    HEIGHT
    HEIGHT_NEW

    ;; peeking flags
    ( PEEK_AT_STACK                     :binary@prove )
    ( PEEK_AT_CONTEXT                   :binary@prove )
    ( PEEK_AT_ACCOUNT                   :binary@prove )
    ( PEEK_AT_STORAGE                   :binary@prove )
    ( PEEK_AT_TRANSACTION               :binary@prove )
    ( PEEK_AT_MISCELLANEOUS             :binary@prove )
    ( PEEK_AT_SCENARIO                  :binary@prove )

    ;; gas columns
    GAS_EXPECTED
    GAS_ACTUAL
    GAS_COST
    GAS_NEXT
    REFGAS
    REFGAS_NEW

    ;; instruction related
    ( TWO_LINE_INSTRUCTION              :binary )        ;; is set by instruction decoding
    ( COUNTER_TLI                       :binary@prove )
    NUMBER_OF_NON_STACK_ROWS
    COUNTER_NSR
)


(defalias
    ;;
    ABS_TX_NUM          ABSOLUTE_TRANSACTION_NUMBER
    BTC_NUM             BATCH_NUMBER
    CMC                 CONTEXT_MAY_CHANGE     
    XAHOY               EXCEPTION_AHOY     
    TX_END_STAMP        HUB_STAMP_TRANSACTION_END
    GAS_XPCT            GAS_EXPECTED
    GAS_ACTL            GAS_ACTUAL
    TLI                 TWO_LINE_INSTRUCTION
    NSR                 NUMBER_OF_NON_STACK_ROWS
    CT_TLI              COUNTER_TLI
    CT_NSR              COUNTER_NSR
    CN                  CONTEXT_NUMBER
    CN_NEW              CONTEXT_NUMBER_NEW
    CALLER_CN           CALLER_CONTEXT_NUMBER
    CN_WILL_REV         CONTEXT_WILL_REVERT
    CN_GETS_REV         CONTEXT_GETS_REVERTED
    CN_SELF_REV         CONTEXT_SELF_REVERTS
    CN_REV_STAMP        CONTEXT_REVERT_STAMP
    CFI                 CODE_FRAGMENT_INDEX
    PC                  PROGRAM_COUNTER
    PC_NEW              PROGRAM_COUNTER_NEW
    ACC                 PEEK_AT_ACCOUNT
    CON                 PEEK_AT_CONTEXT
    SCN                 PEEK_AT_SCENARIO
    STK                 PEEK_AT_STACK
    STO                 PEEK_AT_STORAGE
    TXN                 PEEK_AT_TRANSACTION
    MISC                PEEK_AT_MISCELLANEOUS
    )
