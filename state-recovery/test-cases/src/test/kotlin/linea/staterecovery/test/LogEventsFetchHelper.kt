package linea.staterecovery.test

import linea.domain.BlockParameter
import linea.domain.EthLogEvent
import linea.staterecovery.DataFinalizedV3
import linea.web3j.Web3JLogsSearcher

fun getLastFinalizationOnL1(
  logsSearcher: Web3JLogsSearcher,
  contractAddress: String
): EthLogEvent<DataFinalizedV3> {
  return getFinalizationsOnL1(logsSearcher, contractAddress)
    .lastOrNull()
    ?: error("no finalization found")
}

fun getFinalizationsOnL1(
  logsSearcher: Web3JLogsSearcher,
  contractAddress: String
): List<EthLogEvent<DataFinalizedV3>> {
  return logsSearcher.getLogs(
    fromBlock = BlockParameter.Tag.EARLIEST,
    toBlock = BlockParameter.Tag.LATEST,
    address = contractAddress,
    topics = listOf(DataFinalizedV3.topic)
  ).get().map(DataFinalizedV3.Companion::fromEthLog)
}
