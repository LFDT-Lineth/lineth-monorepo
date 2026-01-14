(module hub)

(defconstraint   generalities---program-counter---stamp-constancy ()
                 (begin
                   (hub-stamp-constancy PC)
                   (hub-stamp-constancy PC_NEW)))

(defconstraint   generalities---program-counter---automatic-vanishing-outside-of-EXEC-phase ()
                 (if-zero TX_EXEC
                          (begin
                            (vanishes! PC)
                            (vanishes! PC_NEW))))

(defconstraint   generalities---program-counter---PC_NEW-vanishes-upon-stack-exception (:guard PEEK_AT_STACK)
                 (if-not-zero    (force-bin   (+    stack/SUX    stack/SOX))
                                 (vanishes!    PC_NEW)))

(defconstraint   generalities---program-counter---automatic-update (:guard PEEK_AT_STACK)
                 (if-zero    (force-bin   (+    stack/SUX    stack/SOX))
                             (if-zero    (force-bin (+ stack/PUSHPOP_FLAG stack/JUMP_FLAG))
                                         (eq! PC_NEW (+ 1 PC)))))
