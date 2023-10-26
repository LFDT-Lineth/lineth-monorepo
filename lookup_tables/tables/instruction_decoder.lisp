(module instruction-decoder)

(defcolumns
    (OPCODE :display :opcode)

    ;;
    ;; Family flags
    ;;
    (FAMILY_ADD :boolean)
    (FAMILY_MOD :boolean)
    (FAMILY_MUL :boolean)
    (FAMILY_EXT :boolean)
    (FAMILY_WCP :boolean)
    (FAMILY_BIN :boolean)
    (FAMILY_SHF :boolean)
    (FAMILY_KEC :boolean)
    (FAMILY_CONTEXT :boolean)
    (FAMILY_ACCOUNT :boolean)
    (FAMILY_COPY :boolean)
    (FAMILY_TRANSACTION :boolean)
    (FAMILY_BATCH :boolean)
    (FAMILY_STACK_RAM :boolean)
    (FAMILY_STORAGE :boolean)
    (FAMILY_JUMP :boolean)
    (FAMILY_MACHINE_STATE :boolean)
    (FAMILY_PUSH_POP :boolean)
    (FAMILY_DUP :boolean)
    (FAMILY_SWAP :boolean)
    (FAMILY_LOG :boolean)
    (FAMILY_CREATE :boolean)
    (FAMILY_CALL :boolean)
    (FAMILY_HALT :boolean)
    (FAMILY_INVALID :boolean)

    ;;
    ;; Stack settings
    ;;
    (PATTERN_ZERO_ZERO :boolean)
    (PATTERN_ONE_ZERO :boolean)
    (PATTERN_TWO_ZERO :boolean)
    (PATTERN_ZERO_ONE :boolean)
    (PATTERN_ONE_ONE :boolean)
    (PATTERN_TWO_ONE :boolean)
    (PATTERN_THREE_ONE :boolean)
    (PATTERN_LOAD_STORE :boolean)
    (PATTERN_DUP :boolean)
    (PATTERN_SWAP :boolean)
    (PATTERN_LOG :boolean)
    (PATTERN_COPY :boolean)
    (PATTERN_CALL :boolean)
    (PATTERN_CREATE :boolean)
    (ALPHA :byte)
    (DELTA :byte)
    (NB_ADDED :byte)
    (NB_REMOVED :byte)
    STATIC_GAS
    (TWO_LINES_INSTRUCTION :boolean)
    (FORBIDDEN_IN_STATIC :boolean)
    (ADDRESS_TRIMMING_INSTRUCTION :boolean)
    (FLAG1 :boolean)
    (FLAG2 :boolean)
    (FLAG3 :boolean)
    (FLAG4 :boolean)

    ;;
    ;; RAM settings
    ;;
    (RAM_ENABLED :boolean)
    (RAM_SOURCE_ROM :boolean)
    (RAM_SOURCE_TXN_DATA :boolean)
    (RAM_SOURCE_RAM :boolean)
    (RAM_SOURCE_STACK :boolean)
    (RAM_SOURCE_EC_DATA :boolean)
    (RAM_SOURCE_EC_INFO :boolean)
    (RAM_SOURCE_MODEXP_DATA :boolean)
    (RAM_SOURCE_HASH_DATA :boolean)
    (RAM_SOURCE_HASH_INFO :boolean)
    (RAM_SOURCE_BLAKE_DATA :boolean)
    (RAM_SOURCE_LOG_DATA :boolean)
    (RAM_TARGET_ROM :boolean)
    (RAM_TARGET_TXN_DATA :boolean)
    (RAM_TARGET_RAM :boolean)
    (RAM_TARGET_STACK :boolean)
    (RAM_TARGET_EC_DATA :boolean)
    (RAM_TARGET_EC_INFO :boolean)
    (RAM_TARGET_MODEXP_DATA :boolean)
    (RAM_TARGET_HASH_DATA :boolean)
    (RAM_TARGET_HASH_INFO :boolean)
    (RAM_TARGET_BLAKE_DATA :boolean)
    (RAM_TARGET_LOG_DATA :boolean)

    ;;
    ;; Billing settings
    ;;
    BILLING_PER_WORD
    BILLING_PER_BYTE
    (MXP_TYPE_1 :boolean)
    (MXP_TYPE_2 :boolean)
    (MXP_TYPE_3 :boolean)
    (MXP_TYPE_4 :boolean)
    (MXP_TYPE_5 :boolean)

    ;;
    ;; ROM columns
    ;;
    (IS_PUSH     :boolean)
    (IS_JUMPDEST :boolean)
    )
