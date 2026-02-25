(module hub)

(defcolumns

    ;; system transactions vs user transaction flags
    ( SYSI                                      :binary@prove )
    ( USER                                      :binary@prove )
    ( SYSF                                      :binary@prove )

    ;; block and transaction accounting
    ( BLK_NUMBER                                :i16 )
    ;; ( TOTL_TXN_NUMBER                           :i16 )
    ( SYSI_TXN_NUMBER                           :i24 )
    ( USER_TXN_NUMBER                           :i24 )
    ( SYSF_TXN_NUMBER                           :i24 )

    ;; transaction processing phases
    ( TX_AUTH                                   :binary@prove )
    ( TX_SKIP                                   :binary@prove )
    ( TX_WARM                                   :binary@prove )
    ( TX_INIT                                   :binary@prove )
    ( TX_EXEC                                   :binary@prove )
    ( TX_FINL                                   :binary@prove )
    ( HUB_STAMP                                 :i32 )
    ( HUB_STAMP_TRANSACTION_END                 :i32 )
    ( CONTEXT_MAY_CHANGE                        :binary@prove )
    ( EXCEPTION_AHOY                            :binary@prove )

    ;; extra stamps
    ( LOG_INFO_STAMP                            :i32 )
    ( MMU_STAMP                                 :i32 )
    ( MXP_STAMP                                 :i32 )

    ;; stamps for undoing operations
    ( DOM_STAMP                                 :i32 )
    ( SUB_STAMP                                 :i32 )

    ;; context data
    ( CONTEXT_NUMBER                            :i32 )
    ( CONTEXT_NUMBER_NEW                        :i32 )
    ( CALLER_CONTEXT_NUMBER                     :i32 )

    ;;
    ( CONTEXT_WILL_REVERT                       :binary )
    ( CONTEXT_GETS_REVERTED                     :binary@prove )
    ( CONTEXT_SELF_REVERTS                      :binary@prove )
    ( CONTEXT_REVERT_STAMP                      :i32 )

    ;;
    ( CODE_FRAGMENT_INDEX                       :i16 )
    ( PROGRAM_COUNTER                           :i24 )
    ( PROGRAM_COUNTER_NEW                       :i24 )
    ( HEIGHT                                    :i11 ) ;; values in the range [0 .. 1024] (inclusive)
    ( HEIGHT_NEW                                :i11 )

    ;; peeking flags
    ( PEEK_AT_AUTHORIZATION                     :binary@prove )
    ( PEEK_AT_ACCOUNT                           :binary@prove )
    ( PEEK_AT_CONTEXT                           :binary@prove )
    ( PEEK_AT_MISCELLANEOUS                     :binary@prove )
    ( PEEK_AT_SCENARIO                          :binary@prove )
    ( PEEK_AT_STACK                             :binary@prove )
    ( PEEK_AT_STORAGE                           :binary@prove )
    ( PEEK_AT_TRANSACTION                       :binary@prove )
    ( PEEK_AT_TRANSIENT                         :binary@prove )

    ;; gas columns
    ( GAS_EXPECTED                              :i64 )
    ( GAS_ACTUAL                                :i64 )
    ( GAS_COST                                  :i64 )
    ( GAS_NEXT                                  :i64 )
    ( REFUND_COUNTER                            :i32 )
    ( REFUND_COUNTER_NEW                        :i32 )

    ;; instruction related
    ( TWO_LINE_INSTRUCTION                      :binary )        ;; is set by instruction decoding
    ( COUNTER_TLI                               :binary@prove )
    ( NON_STACK_ROWS                            :i8 )  ;; maximum value is ~ 20
    ( COUNTER_NSR                               :i8 )  ;; counts from 0, 1 (first "actual" actual value) to NON_STACK_ROWS

)


(defalias
    ;;
    CMC                 CONTEXT_MAY_CHANGE
    XAHOY               EXCEPTION_AHOY
    TX_END_STAMP        HUB_STAMP_TRANSACTION_END
    GAS_XPCT            GAS_EXPECTED
    GAS_ACTL            GAS_ACTUAL
    TLI                 TWO_LINE_INSTRUCTION
    NSR                 NON_STACK_ROWS
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
    AUTH                PEEK_AT_AUTHORIZATION
    CON                 PEEK_AT_CONTEXT
    SCN                 PEEK_AT_SCENARIO
    STK                 PEEK_AT_STACK
    STO                 PEEK_AT_STORAGE
    TXN                 PEEK_AT_TRANSACTION
    MISC                PEEK_AT_MISCELLANEOUS
    )
