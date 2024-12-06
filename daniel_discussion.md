Various variables may undergo changes as we exit, enter or re-enter an execution context. The point of this discussion is to figure out when what changes

General questions:
- what was done before context exit ?
- what happens in between context exit and re-entry ?
- does anything happen between context re-entry and the parent resuming execution ?
- does context-exit only trigger when you definitively exit a context (e.g. not as you leave a context to enter a child context) ?
  - YES, it only happens as the context dies definitively (and returns to its caller / creator / nothingness if it's the root)

The variable stuff
- world state $\sigma$
- machine state $\mu$
- accrued state $A$
- environment $I$

| variable family | variable                     | TraceContextEntry | TraceContextExit | in between                                        | TraceContextReEntry |                                                    |
|-----------------+------------------------------+-------------------+------------------+---------------------------------------------------+---------------------+----------------------------------------------------|
| ACCRUED STATE A |                              |                   |                  |                                                   |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |
|                 | A_s [self destruct set DGAF] |                   |                  |                                                   |                     | "complete method"                                  |
|                 | A_l [logs DGAF]              |                   |                  |                                                   |                     | "complete method"                                  |
|                 | A_t [touched - DGAF]         |                   |                  |                                                   |                     |                                                    |
|                 | A_r [refunds - DGAF]         |                   |                  |                                                   |                     |                                                    |
|                 | A_a [warm addresses]         |                   |                  |                                                   |                     | ∈ frame                                            |
|                 | A_k [warm storage keys]      |                   |                  |                                                   |                     | ∈ frame                                            |
|-----------------+------------------------------+-------------------+------------------+---------------------------------------------------+---------------------+----------------------------------------------------|
| WORLD STATE σ   |                              |                   |                  | here                                              |                     |                                                    |
|                 | accounts                     |                   |                  | here                                              |                     |                                                    |
|                 | - balance                    |                   |                  | here                                              |                     | ∈ worldUpdater                                     |
|                 | - nonce                      |                   |                  | here                                              |                     | ∈ worldUpdater                                     |
|                 | - storage                    |                   |                  | here                                              |                     | ∈ worldUpdater                                     |
|                 | - code                       |                   |                  | here                                              |                     | ∈ worldUpdater, codeSuccess() method               |
|-----------------+------------------------------+-------------------+------------------+---------------------------------------------------+---------------------+----------------------------------------------------|
| MACHINE STATE μ |                              |                   |                  |                                                   |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |
|                 | gas                          |                   |                  | add left over gas to parent gas                   |                     | complete method of the AbstractCallOperation class | complete method of the AbstractCallOperation class |
|                 | pc                           |                   |                  | raise the PC (i.e. do the += 1)                   |                     |                                                    |
|                 | return data                  |                   |                  | set the parent's return data                      |                     |                                                    |
|                 | memory                       |                   |                  |                                                   |                     |                                                    |
|                 | active words in memory       |                   |                  |                                                   |                     |                                                    |
|                 | stack                        |                   |                  | add successBit / deploymentAddress onto the stack |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |
|                 |                              |                   |                  |                                                   |                     |                                                    |

In particular the difference between CALL / CREATE. As per our discussion the other day an unexceptional CALL immediately adds the targetAddress into A_a --- even if the CALL is aborted (insufficientBalance/maxCallStackDepthReached). For CREATE's this only happens if you enter Λ, the deployment function. So it won't happen if you get aborted (same reasons) but it will happen if you trigger the failure condition F or execute a normal deployment.


AbstractMessageProcessor
    tracecontextExit
        completedSuccess (you transmit the WorldStateUpdates to the parent)
        completedFailure

Deployment
- exception
- aborted (balance || call stack depth)
- failure condition
- entry:
  - empty initialization code:
  - nonempty initialization code:
          NOTE: We have questions such as: will the balance transfer + creator nonce update done at TracePostExecution ?
          This is particularly important for the empty init code case (since THE ARITHMETIZATION won't be entering a new
          context, using TraceContextEnter is not a viable option in this case to retrieve the correct account snapshots)
          
          If a CREATE(2) produces a failure condition (...) does Besu create and enter a child frame ?
          Same question for empty initialization code.
          Our reason for asking is that:
            - for failure condition and empty init code we would like to take snapshots of the updated creator and createe
              accounts when the TraceContextReEntry hook is activated. If not "re-entry" takes place we must hook somewhere
              else ... maybe post execution of the current OpCode ... or even PreExecution of the next OpCode ... ?
          Other question:
            - we remember from discussions almost a year ago with @Franklin and @Daniel that when Besu either
              * did a CALL to an account with empty bytecode that BESU executes a STOP
              * did a CREATE with empty init code that, again, BESU would execute a STOP
            (both in compliance with the Yellow Paper.) Is that indeed the case ? I remember that I asked @Franklin to make it
            so that our arithmetization would not trace that STOP. But that would lead

CREATOR:
- decrement balance: 
- increment nonce:                happens before the contextEntry (normal)

CREATEE:
- raising of the creator nonce    (0 → 1)
- balance update of creator / createe happens after contextEntry (normal)
- at contextEnter we don't have the updated accounts (balances / nonces / code)



If a CREATE(2) produces a failure condition (...) does Besu create and enter a child frame ?
Same question for empty initialization code.
Our reason for asking is that:
  - for failure condition and empty init code we would like to take snapshots of the updated creator and createe
    accounts when the TraceContextReEntry hook is activated. If not "re-entry" takes place we must hook somewhere
    else ... maybe post execution of the current OpCode ... or even PreExecution of the next OpCode ... ?
Other question:
  - we remember from discussions almost a year ago with @Franklin and @Daniel that when Besu either
    * did a CALL to an account with empty bytecode that BESU executes a STOP
    * did a CREATE with empty init code that, again, BESU would execute a STOP
  (both in compliance with the Yellow Paper.) Is that indeed the case ? I remember that I asked @Franklin to make it
  so that our arithmetization would not trace that STOP. But that would lead
