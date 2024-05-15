(module hub)

(defconstraint PC-stamp-constancy ()
               (begin
                 (hub-stamp-constancy PC)
                 (hub-stamp-constancy PC_NEW)))

(defconstraint PC-automatic-vanishing-outside-of-EXEC-phase ()
               (if-zero TX_EXEC
                        (begin
                          (vanishes! PC)
                          (vanishes! PC_NEW))))

(defconstraint PC-automatic-update (:guard PEEK_AT_STACK)
               (if-zero (force-bin (+ stack/PUSHPOP_FLAG stack/JUMP_FLAG))
                        (eq! PC_NEW (+ 1 PC))))
