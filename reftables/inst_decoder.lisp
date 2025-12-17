(module instdecoder)

(defcolumns
    (OPCODE :byte :display :opcode)

    ;;
    ;; Family flags
    ;;
    (FAMILY_ADD                            :binary)
    (FAMILY_MOD                            :binary)
    (FAMILY_MUL                            :binary)
    (FAMILY_EXT                            :binary)
    (FAMILY_WCP                            :binary)
    (FAMILY_BIN                            :binary)
    (FAMILY_SHF                            :binary)
    (FAMILY_KEC                            :binary)
    (FAMILY_CONTEXT                        :binary)
    (FAMILY_ACCOUNT                        :binary)
    (FAMILY_COPY                           :binary)
    (FAMILY_MCOPY                          :binary)
    (FAMILY_TRANSACTION                    :binary)
    (FAMILY_BATCH                          :binary)
    (FAMILY_STACK_RAM                      :binary)
    (FAMILY_STORAGE                        :binary)
    (FAMILY_TRANSIENT                      :binary)
    (FAMILY_JUMP                           :binary)
    (FAMILY_MACHINE_STATE                  :binary)
    (FAMILY_PUSH_POP                       :binary)
    (FAMILY_DUP                            :binary)
    (FAMILY_SWAP                           :binary)
    (FAMILY_LOG                            :binary)
    (FAMILY_CREATE                         :binary)
    (FAMILY_CALL                           :binary)
    (FAMILY_HALT                           :binary)
    (FAMILY_INVALID                        :binary)
    (TWO_LINE_INSTRUCTION                  :binary)
    (STATIC_FLAG                           :binary)
    (MXP_FLAG                              :binary)
    (FLAG_1                                :binary)
    (FLAG_2                                :binary)
    (FLAG_3                                :binary)
    (FLAG_4                                :binary)
    (ALPHA                                 :byte)
    (DELTA                                 :byte)
    (STATIC_GAS                            :i32)

    ;;
    ;; Billing settings
    ;;
    (BILLING_PER_WORD     :byte)
    (BILLING_PER_BYTE     :byte)
    (IS_MSIZE             :binary)
    (IS_RETURN            :binary)
    (IS_MCOPY             :binary)
    (IS_FIXED_SIZE_1      :binary)
    (IS_FIXED_SIZE_32     :binary)
    (IS_SINGLE_MAX_OFFSET :binary)
    (IS_DOUBLE_MAX_OFFSET :binary)
    (IS_WORD_PRICING      :binary)
    (IS_BYTE_PRICING      :binary)

    ;;
    ;; ROM columns
    ;;
    (IS_PUSH     :binary)
    (IS_JUMPDEST :binary)
    )
