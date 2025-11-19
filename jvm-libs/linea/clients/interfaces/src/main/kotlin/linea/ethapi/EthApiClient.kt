package linea.ethapi

/**
 * Failed requests with JSON-RPC error responses will be rejected with JsonRpcErrorResponseException
 */
interface EthApiClient :
  EthApiChainIdClient,
  EthApiFeeClient,
  EthApiSimulationClient,
  EthApiBlockClient,
  EthLogsClient,
  EthApiAccountClient,
  EthApiTransactionClient,
  EthApiExecutionClientInfo
// future methods to eventually add if necessary
// fun ethGetCode(address: ByteArray, blockParameter: BlockParameter): SafeFuture<ByteArray>
// fun ethGetStorageAt(address: ByteArray, position: BigInteger, blockParameter: BlockParameter): SafeFuture<ByteArray>
// fun ethGetBlockTransactionCountByHash(blockHash: ByteArray): SafeFuture<ULong?>
// fun ethGetBlockTransactionCountByNumber(blockParameter: BlockParameter): SafeFuture<ULong?>
