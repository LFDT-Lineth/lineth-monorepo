(module mxp)

(defcolumns
	STAMP
	CN
	CT
	(ROOB	:binary)
	(NOOP	:binary)
	(MXPX	:binary)
	(INST   :display :opcode)
	(MXP_TYPE :binary :array[5])
	GBYTE
	GWORD
	(DEPLOYS :binary)
	OFFSET_1_LO
	OFFSET_2_LO
	OFFSET_1_HI
	OFFSET_2_HI
	SIZE_1_LO
	SIZE_2_LO
	SIZE_1_HI
	SIZE_2_HI
	MAX_OFFSET_1
	MAX_OFFSET_2
	MAX_OFFSET
	(COMP :binary)	
	(BYTE :byte :array[4])
	(BYTE_A	:byte)
	(BYTE_W	:byte)
	(BYTE_Q	:byte)
	(ACC :array[4])
	ACC_A
	ACC_W
	ACC_Q
	BYTE_QQ
	BYTE_R
	WORDS
	WORDS_NEW
	C_MEM
	C_MEM_NEW
	QUAD_COST
	LIN_COST
	GAS_MXP
	(EXPANDS :binary))
