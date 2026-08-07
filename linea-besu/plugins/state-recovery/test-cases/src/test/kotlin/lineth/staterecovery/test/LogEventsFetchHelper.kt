package lineth.staterecovery.test

import lineth.contract.events.DataFinalizedV3
import lineth.domain.BlockParameter
import lineth.domain.EthLogEvent
import lineth.ethapi.EthLogsSearcherImpl

fun getLastFinalizationOnL1(logsSearcher: EthLogsSearcherImpl, contractAddress: String): EthLogEvent<DataFinalizedV3> {
  return getFinalizationsOnL1(logsSearcher, contractAddress)
    .lastOrNull()
    ?: error("no finalization found")
}

fun getFinalizationsOnL1(
  logsSearcher: EthLogsSearcherImpl,
  contractAddress: String,
): List<EthLogEvent<DataFinalizedV3>> {
  return logsSearcher.getLogs(
    fromBlock = BlockParameter.Tag.EARLIEST,
    toBlock = BlockParameter.Tag.LATEST,
    address = contractAddress,
    topics = listOf(DataFinalizedV3.topic),
  ).get().map(DataFinalizedV3.Companion::fromEthLog)
}
