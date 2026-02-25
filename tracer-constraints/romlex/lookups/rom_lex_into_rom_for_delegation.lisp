(defclookup
  romlex-into-rom-for-delegation
  ;; target columns
  (
   rom.CODE_FRAGMENT_INDEX
   (next rom.PROGRAM_COUNTER)
   (shift rom.ACC 2)
   (four-bytes-lead-bytes)
   (sixteen-bytes-tail-bytes)
   )
  ;; source selector
  romlex.COULD_BE_DELEGATION_CODE
  ;; source columns
  (
   romlex.CODE_FRAGMENT_INDEX
   1
   romlex.LEADING_THREE_BYTES
   romlex.LEAD_DELEGATION_BYTES
   romlex.TAIL_DELEGATION_BYTES
   ))

(defun (four-bytes-lead-bytes)
  (+ (* (^ 256 3) (shift rom.PADDED_BYTECODE_BYTE 3) )
     (* (^ 256 2) (shift rom.PADDED_BYTECODE_BYTE 4) )
     (* (^ 256 1) (shift rom.PADDED_BYTECODE_BYTE 5) )
     (* (^ 256 0) (shift rom.PADDED_BYTECODE_BYTE 6) )
     ))

(defun (sixteen-bytes-tail-bytes)
  (+ (* (^ 256 15) (shift rom.PADDED_BYTECODE_BYTE 7 ) )
     (* (^ 256 14) (shift rom.PADDED_BYTECODE_BYTE 8 ) )
     (* (^ 256 13) (shift rom.PADDED_BYTECODE_BYTE 9 ) )
     (* (^ 256 12) (shift rom.PADDED_BYTECODE_BYTE 10) )
     (* (^ 256 11) (shift rom.PADDED_BYTECODE_BYTE 11) )
     (* (^ 256 10) (shift rom.PADDED_BYTECODE_BYTE 12) )
     (* (^ 256 9 ) (shift rom.PADDED_BYTECODE_BYTE 13) )
     (* (^ 256 8 ) (shift rom.PADDED_BYTECODE_BYTE 14) )
     (* (^ 256 7 ) (shift rom.PADDED_BYTECODE_BYTE 15) )
     (* (^ 256 6 ) (shift rom.PADDED_BYTECODE_BYTE 16) )
     (* (^ 256 5 ) (shift rom.PADDED_BYTECODE_BYTE 17) )
     (* (^ 256 4 ) (shift rom.PADDED_BYTECODE_BYTE 18) )
     (* (^ 256 3 ) (shift rom.PADDED_BYTECODE_BYTE 19) )
     (* (^ 256 2 ) (shift rom.PADDED_BYTECODE_BYTE 20) )
     (* (^ 256 1 ) (shift rom.PADDED_BYTECODE_BYTE 21) )
     (* (^ 256 0 ) (shift rom.PADDED_BYTECODE_BYTE 22) )
     ))
