(module rlp)

(defcolumns
  ;;
  ;; INPUTS
  ;;
  (ADDR_HI :display :hex) ;; hi part (4B)  of the creator address
  (ADDR_LO :display :hex) ;; lo part (16B) "
  (NONCE :display :hex)   ;; nonce (1-8B)  "
  STAMP


  ;;
  ;; Register columns
  ;;
  ct                          ;; rhytm of an RLP encoding operation (16 lines)

  (addr_lo_1 :display :hex)   ;; piece of ADDR_LO stored in OUT_1 (10B)
  (addr_lo_2 :display :hex)   ;; piece of ADDR_LO   "    "  OUT_2 (6B)
  (addr_lo_ax :display :hex)  ;; register for addr_lo_{1,2} construction
  (addr_lo_ndl :bool)         ;; indicates whether addr_lo_1 (= 0) or addr_lo_2 (= 1) is being built

  tn                         ;; line-wise bit decomposition of NONCE least significant byte
  (in-nonce :bool)           ;; set to 1 if the first non-null byte of the nonce has been found
  NONCE_ax                   ;; register for NONCE reconstruction
  NONCE_bytes                ;; leading-0s trimmed bytes of NONCE
  NONCE_n                    ;; effective number of bytes in NONCE (e.g. without the leading 0s)
  (out2_shift :display :hex) ;; shift required to place the nonce in OUT/2; i.e. 256^N_n

  ;;
  ;; OUTPUTS
  ;;
  (OUT :display :hex)  ;; bytes of the output
  N_BYTES)             ;; the number of bytes to read
