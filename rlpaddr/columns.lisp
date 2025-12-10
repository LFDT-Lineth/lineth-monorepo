(module rlpaddr)

(defcolumns
  ;; INPUTS
  (RECIPE :byte)
  (RECIPE_1 :binary@prove)
  (RECIPE_2 :binary@prove)
  (ADDR_HI :i32)      ;; hi part (4B)  of the creator address
  (ADDR_LO :i128)     ;; lo part (16B) "
  (DEP_ADDR_HI :i32)  ;; hi part of the deploed addr
  (DEP_ADDR_LO :i128) ;; lo part of "
  (RAW_ADDR_HI :i128)
  (NONCE :i64)        ;; nonce (1-8B)  "
  (SALT_HI :i128)
  (SALT_LO :i128)
  (KEC_HI :i128)
  (KEC_LO :i128)
  ;; OUTPUTS
  (LIMB :i128)        ;; bytes of the output
  (LC :binary@prove)
  (nBYTES :byte)      ;; the number of bytes to read
  (INDEX :byte)
  ;; Register columns
  (STAMP :i16)
  (COUNTER :byte)
  (BYTE1 :byte@prove)
  (ACC :i64)
  (ACC_BYTESIZE :byte)
  (POWER :i128)
  (BIT1 :binary@prove)
  (BIT_ACC :byte)
  (TINY_NON_ZERO_NONCE :binary@prove)
  (SELECTOR_KECCAK_RES :binary))

;; aliases
(defalias
  ct COUNTER)


