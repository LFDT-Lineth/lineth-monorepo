(module mmu)

;1.
(defconstraint byte-decompositions ()
				(if-zero IS_MICRO
					(begin
						(byte-decomposition CT ACC_1 BYTE_1)
						(byte-decomposition CT ACC_2 BYTE_2)
						(byte-decomposition CT ACC_3 BYTE_3)
						(byte-decomposition CT ACC_4 BYTE_4)
						(byte-decomposition CT ACC_5 BYTE_5)
						(byte-decomposition CT ACC_6 BYTE_6)
						(byte-decomposition CT ACC_7 BYTE_7)
						(byte-decomposition CT ACC_8 BYTE_8))))