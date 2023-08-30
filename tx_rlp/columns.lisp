(module txRlp)

(defcolumns
    ABS_TX_NUM
    LIMB
    (nBYTES :byte)
    (LIMB_CONSTRUCTED	:boolean)
    (LT	:boolean)
    (LX	:boolean)
    INDEX_LT
    INDEX_LX
    (PHASE	:boolean	:array[0:14])
    (end_phase	:boolean)
    (TYPE	:byte)
    (OLI	:boolean)
    (COUNTER	:byte)
    (DONE	:boolean)
    (number_step	:byte)
    (INPUT	:display :bytes :array[2])
    (BYTE :byte :array[2])
    (ACC :display :bytes :array[2])
    ACC_BYTESIZE
    (BIT	:boolean)
    (BIT_ACC	:byte)
    POWER
    (is_bytesize	:boolean)
    (is_list	:boolean)
    (COMP	:boolean)
    RLP_LT_BYTESIZE
    RLP_LX_BYTESIZE
    (is_padding	:boolean)
    (is_prefix	:boolean)
    PHASE_BYTESIZE
    INDEX_DATA
    DATAGASCOST
    (DEPTH	:boolean	:array[2])
    ADDR_HI
    ADDR_LO
    ACCESS_TUPLE_BYTESIZE
    nb_Addr
    nb_Sto
    nb_Sto_per_Addr)

;; aliases
(defalias
    CT			COUNTER
    LC          LIMB_CONSTRUCTED
    P           POWER)
