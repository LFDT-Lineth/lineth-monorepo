(module hash_data)

(defcolumns
    INST
    NUM
    INDEX
    LIMB)

(defconstraint heartbeat-init (:domain {0}) (vanishes! NUM))

(defconstraint heartbeat-global ()
  (begin
   (num-zero-implies-zero NUM INDEX)
   (num-zero-implies-zero NUM LIMB)
   (num-zero-implies-zero NUM INST)
   (num-non-decreasing NUM)
   (index-grows-or-resets NUM INDEX)))
