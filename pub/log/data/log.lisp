(module log_data)

(defcolumns
    NUM
    INDEX
    LIMB)

(defconstraint heartbeat-init (:domain {0}) (vanishes NUM))

(defconstraint heartbeat-global ()
  (begin
   (num-zero-implies-zero NUM INDEX)
   (num-zero-implies-zero NUM LIMB)
   (num-non-decreasing NUM)
   (index-grows-or-resets NUM INDEX)))
