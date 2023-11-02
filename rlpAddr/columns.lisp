(module rlpAddr)

(defcolumns 
  ;; INPUTS
  RECIPE
  (RECIPE_1 :binary)
  (RECIPE_2 :binary)
  ADDR_HI     ;; hi part (4B)  of the creator address
  ADDR_LO     ;; lo part (16B) "
  DEP_ADDR_HI ;; hi part of the deploed addr
  DEP_ADDR_LO ;; lo part of "
  NONCE       ;; nonce (1-8B)  "
  SALT_HI
  SALT_LO
  KEC_HI
  KEC_LO
  ;; OUTPUTS
  LIMB        ;; bytes of the output
  (LC :binary)
  nBYTES      ;; the number of bytes to read
  INDEX
  ;; Register columns
  STAMP
  COUNTER
  (BYTE1 :byte)
  ACC
  ACC_BYTESIZE
  POWER
  (BIT1 :binary)
  (BIT_ACC :byte)
  (TINY_NON_ZERO_NONCE :binary))

;; aliases
(defalias 
  ct COUNTER)


