package net.consensys.zkevm.load.model.inner

class Context(val chainId: Int, val contracts: List<CreateContract>, val url: String, val nbOfExecutions: Int)
