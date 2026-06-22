## Summary

Add a `LogBus` query type to `prover-ray` to support the logup message-bus of the R5 VM, which is used as the inter-table communication mechanism.

Semantically, the LogUp bus can be seen as a tool in the implementation of conditional fractional lookups (at a high level). If we want to perform a fractional lookup over source tables S1, S2, S3 and target tables T1, T2, T3, where each (S_i, T_i) belongs to a different segment i, the semantics of the fractional lookup are expressed by the relation S1∪S2∪S3⊆T1∪T2∪T3.  
When the tables belong to the same segment, we do not need to provide multiplicity information to the LogUp bus query .
LogUp bus queries can be complete or incomplete, depending on whether the log derivative sum is zero or non-zero. Both will be supported. 

## API

`go-corset` is expected to surface this via the `binfile` with roughly the following shape:

```go
Multiplicities []Column // multiplicity columns at the receiving end
Sender         []Table  // sending end of the bus
Receiver       []Table  // receiving end of the bus
ResultClaim    ...      // claim produced by the bus
```

In GoCorset, we need an additional query for the LogUp bus. LogUp bus queries can be associated to handles. Similarly to lookups, we write source and target tables, which are then sent and received to/from a LogUp handle.

At some point, LogUp queries end up being compiled into a single unified query. This can be done on the arithmetization side, by clustering multiple function calls using padding and selectors. However, for debugging purposes, we could keep different bus queries on the arithmetization side and in corset, leaving them to be compiled later in the prover. 

### Multiplicity: 
The multiplicity can be represented as an optional column given to the LogUp query. 

## Multiplicity Concerns:
It might be that there exist cases where the multiplicity exceeds 2^31. A potential solution would be to split large multiplicities into multiple trace entries. To do: flag any cases in which this happens in practice. It might not be an issue cryptographically if everything remains on KoalaBear elements, but this remains to be discussed. 
Computing a lot of multiplicity columns is expected to have an impact on performance, by affecting the latency of the fast tracer. When appropriate, replace bus queries with local lookups for static tables (by replicating small reference tables across segments). 

### Note for registers: 
Olivier clarified that register files don't need multiplicity due to timestamping.







