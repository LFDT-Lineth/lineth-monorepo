(module rlptxn)

(defun (rlputils-call---INTEGER   relOffset
                                  integer-hi
                                  integer-lo)
  (begin 
    (eq! (shift cmp/RLPUTILS_FLAG   relOffset) 1)
    (eq! (shift cmp/RLPUTILS_INST   relOffset) RLP_UTILS_INST_INTEGER)
    (eq! (shift cmp/EXO_DATA_1      relOffset) integer-hi)
    (eq! (shift cmp/EXO_DATA_2      relOffset) integer-lo)))

(defun (rlputils-call---BYTE_STRING_PREFIX-non-trivial   relOffset
                                                         length
                                                         is-list)
  (begin 
    (eq! (shift cmp/RLPUTILS_FLAG   relOffset) 1)
    (eq! (shift cmp/RLPUTILS_INST   relOffset) RLP_UTILS_INST_BYTE_STRING_PREFIX)
    (eq! (shift cmp/EXO_DATA_1      relOffset) length)
    ;; (eq! (shift cmp/EXO_DATA_2      relOffset) first-byte)
    (eq! (shift cmp/EXO_DATA_3      relOffset) is-list)))

(defun (rlputils-call---BYTE_STRING_PREFIX   relOffset
                                             length
                                             first-byte
                                             is-list)
  (begin 
    (eq! (shift cmp/RLPUTILS_FLAG   relOffset) 1)
    (eq! (shift cmp/RLPUTILS_INST   relOffset) RLP_UTILS_INST_BYTE_STRING_PREFIX)
    (eq! (shift cmp/EXO_DATA_1      relOffset) length)
    (eq! (shift cmp/EXO_DATA_2      relOffset) first-byte)
    (eq! (shift cmp/EXO_DATA_3      relOffset) is-list)))

(defun (rlputils-call---BYTES32   relOffset
                                  data-hi
                                  data-lo)
  (begin 
    (eq! (shift cmp/RLPUTILS_FLAG   relOffset) 1)
    (eq! (shift cmp/RLPUTILS_INST   relOffset) RLP_UTILS_INST_BYTES32)
    (eq! (shift cmp/EXO_DATA_1      relOffset) data-hi)
    (eq! (shift cmp/EXO_DATA_2      relOffset) data-lo)))

(defun (rlputils-call---DATA_PRICING   relOffset
                                     limb
                                     n-bytes)
  (begin 
    (eq! (shift cmp/RLPUTILS_FLAG   relOffset) 1)
    (eq! (shift cmp/RLPUTILS_INST   relOffset) RLP_UTILS_INST_DATA_PRICING)
    (eq! (shift cmp/EXO_DATA_1      relOffset) limb)
    (eq! (shift cmp/EXO_DATA_2      relOffset) n-bytes)))
