(module instruction-decoder)

(defcolumns
    (OPCODE :display :opcode)

    ;;
    ;; Family flags
    ;;
    (FAMILY_ADD :binary)
    (FAMILY_MOD :binary)
    (FAMILY_MUL :binary)
    (FAMILY_EXT :binary)
    (FAMILY_WCP :binary)
    (FAMILY_BIN :binary)
    (FAMILY_SHF :binary)
    (FAMILY_KEC :binary)
    (FAMILY_CONTEXT :binary)
    (FAMILY_ACCOUNT :binary)
    (FAMILY_COPY :binary)
    (FAMILY_TRANSACTION :binary)
    (FAMILY_BATCH :binary)
    (FAMILY_STACK_RAM :binary)
    (FAMILY_STORAGE :binary)
    (FAMILY_JUMP :binary)
    (FAMILY_MACHINE_STATE :binary)
    (FAMILY_PUSH_POP :binary)
    (FAMILY_DUP :binary)
    (FAMILY_SWAP :binary)
    (FAMILY_LOG :binary)
    (FAMILY_CREATE :binary)
    (FAMILY_CALL :binary)
    (FAMILY_HALT :binary)
    (FAMILY_INVALID :binary)

    ;;
    ;; Stack settings
    ;;
    (PATTERN_ZERO_ZERO :binary)
    (PATTERN_ONE_ZERO :binary)
    (PATTERN_TWO_ZERO :binary)
    (PATTERN_ZERO_ONE :binary)
    (PATTERN_ONE_ONE :binary)
    (PATTERN_TWO_ONE :binary)
    (PATTERN_THREE_ONE :binary)
    (PATTERN_LOAD_STORE :binary)
    (PATTERN_DUP :binary)
    (PATTERN_SWAP :binary)
    (PATTERN_LOG :binary)
    (PATTERN_COPY :binary)
    (PATTERN_CALL :binary)
    (PATTERN_CREATE :binary)
    (ALPHA :byte)
    (DELTA :byte)
    (NB_ADDED :byte)
    (NB_REMOVED :byte)
    STATIC_GAS
    (TWO_LINES_INSTRUCTION :binary)
    (FORBIDDEN_IN_STATIC :binary)
    (ADDRESS_TRIMMING_INSTRUCTION :binary)
    (FLAG1 :binary)
    (FLAG2 :binary)
    (FLAG3 :binary)
    (FLAG4 :binary)

    ;;
    ;; RAM settings
    ;;
    (RAM_ENABLED :binary)
    (RAM_SOURCE_ROM :binary)
    (RAM_SOURCE_TXN_DATA :binary)
    (RAM_SOURCE_RAM :binary)
    (RAM_SOURCE_STACK :binary)
    (RAM_SOURCE_EC_DATA :binary)
    (RAM_SOURCE_EC_INFO :binary)
    (RAM_SOURCE_MODEXP_DATA :binary)
    (RAM_SOURCE_HASH_DATA :binary)
    (RAM_SOURCE_HASH_INFO :binary)
    (RAM_SOURCE_BLAKE_DATA :binary)
    (RAM_SOURCE_LOG_DATA :binary)
    (RAM_TARGET_ROM :binary)
    (RAM_TARGET_TXN_DATA :binary)
    (RAM_TARGET_RAM :binary)
    (RAM_TARGET_STACK :binary)
    (RAM_TARGET_EC_DATA :binary)
    (RAM_TARGET_EC_INFO :binary)
    (RAM_TARGET_MODEXP_DATA :binary)
    (RAM_TARGET_HASH_DATA :binary)
    (RAM_TARGET_HASH_INFO :binary)
    (RAM_TARGET_BLAKE_DATA :binary)
    (RAM_TARGET_LOG_DATA :binary)

    ;;
    ;; Billing settings
    ;;
    BILLING_PER_WORD
    BILLING_PER_BYTE
    (MXP_TYPE_1 :binary)
    (MXP_TYPE_2 :binary)
    (MXP_TYPE_3 :binary)
    (MXP_TYPE_4 :binary)
    (MXP_TYPE_5 :binary)

    ;;
    ;; ROM columns
    ;;
    (IS_PUSH     :binary)
    (IS_JUMPDEST :binary)
    )
