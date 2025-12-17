package net.consensys.linea.transactionexclusion.app.api

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.get
import com.github.michaelbull.result.getError
import io.vertx.core.json.JsonObject
import linea.kotlin.encodeHex
import linea.kotlin.toHexString
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcRequestMapParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.transactionexclusion.ErrorType
import net.consensys.linea.transactionexclusion.TransactionExclusionError
import net.consensys.linea.transactionexclusion.TransactionExclusionServiceV1
import net.consensys.linea.transactionexclusion.test.defaultRejectedTransaction
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

class RequestHandlersTest {
  private lateinit var transactionExclusionServiceMock: TransactionExclusionServiceV1

  private val mapParams = mapOf(
    "txRejectionStage" to "SEQUENCER",
    "timestamp" to "2024-09-05T09:22:52Z",
    "transactionRLP" to defaultRejectedTransaction.transactionRLP.encodeHex(),
    "reasonMessage" to defaultRejectedTransaction.reasonMessage,
    "overflows" to defaultRejectedTransaction.overflows,
  )

  private val mapRequest = JsonRpcRequestMapParams(
    "2.0",
    "1",
    "linea_saveRejectedTransactionV1",
    mapParams,
  )

  private val listRequest = JsonRpcRequestListParams(
    "2.0",
    "1",
    "linea_saveRejectedTransactionV1",
    listOf(mapParams),
  )

  @BeforeEach
  fun beforeEach() {
    transactionExclusionServiceMock = mock<TransactionExclusionServiceV1>(
      defaultAnswer = Mockito.RETURNS_DEEP_STUBS,
    )
  }

  @Test
  fun saveRejectedTransactionRequestHandlerV1_rejectsEmptyMap() {
    val request = JsonRpcRequestMapParams("", "", "", emptyMap<String, Any>())

    val saveRequestHandlerV1 = SaveRejectedTransactionRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val result = saveRequestHandlerV1.invoke(
      user = null,
      request = request,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(
      Err(
        JsonRpcErrorResponse.invalidParams(
          request.id,
          "Missing [txRejectionStage,timestamp,reasonMessage,transactionRLP,overflows] " +
            "from the given request params",
        ),
      ),
      result,
    )
  }

  @Test
  fun saveRejectedTransactionRequestHandlerV1_rejectsEmptyList() {
    val request = JsonRpcRequestListParams("", "", "", emptyList())

    val saveRequestHandlerV1 = SaveRejectedTransactionRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val result = saveRequestHandlerV1.invoke(
      user = null,
      request = request,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(
      Err(
        JsonRpcErrorResponse.invalidParams(
          request.id,
          "The given request params list should have one argument",
        ),
      ),
      result,
    )
  }

  @Test
  fun saveRejectedTransactionRequestHandlerV1_rejectsListWithInvalidArgument() {
    val request = JsonRpcRequestListParams("", "", "", listOf("invalid_argument"))

    val saveRequestHandlerV1 = SaveRejectedTransactionRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val result = saveRequestHandlerV1.invoke(
      user = null,
      request = request,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(
      Err(
        JsonRpcErrorResponse.invalidParams(
          request.id,
          "The argument in the request params list should be an object",
        ),
      ),
      result,
    )
  }

  @Test
  fun saveRejectedTransactionRequestHandlerV1_invoke_acceptsValidRequestMap() {
    whenever(transactionExclusionServiceMock.saveRejectedTransaction(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          Ok(TransactionExclusionServiceV1.SaveRejectedTransactionStatus.SAVED),
        ),
      )

    val saveRequestHandlerV1 = SaveRejectedTransactionRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val expectedResult = JsonObject()
      .put("status", TransactionExclusionServiceV1.SaveRejectedTransactionStatus.SAVED)
      .put("txHash", defaultRejectedTransaction.transactionInfo.hash.encodeHex())
      .let {
        JsonRpcSuccessResponse(mapRequest.id, it)
      }

    val result = saveRequestHandlerV1.invoke(
      user = null,
      request = mapRequest,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(expectedResult, result.get())
  }

  @Test
  fun saveRejectedTransactionRequestHandlerV1_invoke_acceptsValidRequestList() {
    whenever(transactionExclusionServiceMock.saveRejectedTransaction(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          Ok(TransactionExclusionServiceV1.SaveRejectedTransactionStatus.SAVED),
        ),
      )

    val saveRequestHandlerV1 = SaveRejectedTransactionRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val expectedResult = JsonObject()
      .put("status", TransactionExclusionServiceV1.SaveRejectedTransactionStatus.SAVED)
      .put("txHash", defaultRejectedTransaction.transactionInfo.hash.encodeHex())
      .let {
        JsonRpcSuccessResponse(listRequest.id, it)
      }

    val result = saveRequestHandlerV1.invoke(
      user = null,
      request = listRequest,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(expectedResult, result.get())
  }

  @Test
  fun saveRejectedTransactionRequestHandlerV1_invoke_acceptsValidRequestMap_without_blockNumber() {
    whenever(transactionExclusionServiceMock.saveRejectedTransaction(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          Ok(TransactionExclusionServiceV1.SaveRejectedTransactionStatus.SAVED),
        ),
      )

    val saveTxRequestHandlerV1 = SaveRejectedTransactionRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val expectedResult = JsonObject()
      .put("status", TransactionExclusionServiceV1.SaveRejectedTransactionStatus.SAVED)
      .put("txHash", defaultRejectedTransaction.transactionInfo.hash.encodeHex())
      .let {
        JsonRpcSuccessResponse(mapRequest.id, it)
      }

    val result = saveTxRequestHandlerV1.invoke(
      user = null,
      request = mapRequest,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(expectedResult, result.get())
  }

  @Test
  fun saveRejectedTransactionRequestHandlerV1_invoke_return_success_result_with_duplicate_status() {
    whenever(transactionExclusionServiceMock.saveRejectedTransaction(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          Ok(TransactionExclusionServiceV1.SaveRejectedTransactionStatus.DUPLICATE_ALREADY_SAVED_BEFORE),
        ),
      )

    val saveTxRequestHandlerV1 = SaveRejectedTransactionRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val expectedResult = JsonObject()
      .put("status", TransactionExclusionServiceV1.SaveRejectedTransactionStatus.DUPLICATE_ALREADY_SAVED_BEFORE)
      .put("txHash", defaultRejectedTransaction.transactionInfo.hash.encodeHex())
      .let {
        JsonRpcSuccessResponse(mapRequest.id, it)
      }

    val result = saveTxRequestHandlerV1.invoke(
      user = null,
      request = mapRequest,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(expectedResult, result.get())
  }

  @Test
  fun saveRejectedTransactionRequestHandlerV1_invoke_return_failure_result() {
    whenever(transactionExclusionServiceMock.saveRejectedTransaction(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          Err(
            TransactionExclusionError(
              ErrorType.SERVER_ERROR,
              "error for unit test",
            ),
          ),
        ),
      )

    val saveTxRequestHandlerV1 = SaveRejectedTransactionRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val expectedResult = JsonRpcErrorResponse(
      mapRequest.id,
      jsonRpcError(
        TransactionExclusionError(
          ErrorType.SERVER_ERROR,
          "error for unit test",
        ),
      ),
    )

    val result = saveTxRequestHandlerV1.invoke(
      user = null,
      request = mapRequest,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(expectedResult, result.getError())
  }

  @Test
  fun getTransactionExclusionStatusRequestHandlerV1_rejectsEmptyList() {
    val request = JsonRpcRequestListParams("", "", "", emptyList())

    val getRequestHandlerV1 = GetTransactionExclusionStatusRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val result = getRequestHandlerV1.invoke(
      user = null,
      request = request,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(
      Err(
        JsonRpcErrorResponse.invalidParams(
          request.id,
          "The given request params list should have one argument",
        ),
      ),
      result,
    )
  }

  @Test
  fun getTransactionExclusionStatusRequestHandlerV1_rejectsListWithInvalidArgument() {
    val request = JsonRpcRequestListParams("", "", "", listOf("0x123"))

    val getRequestHandlerV1 = GetTransactionExclusionStatusRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val result = getRequestHandlerV1.invoke(
      user = null,
      request = request,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(
      Err(
        JsonRpcErrorResponse.invalidParams(
          request.id,
          "Hex string of transaction hash cannot be parsed: Must have an even length",
        ),
      ),
      result,
    )
  }

  @Test
  fun getTransactionExclusionStatusRequestHandlerV1_invoke_acceptsValidRequestList() {
    whenever(transactionExclusionServiceMock.getTransactionExclusionStatus(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          Ok(defaultRejectedTransaction),
        ),
      )

    val request = JsonRpcRequestListParams(
      "2.0",
      "1",
      "linea_getTransactionExclusionStatusV1",
      listOf(
        defaultRejectedTransaction.transactionInfo.hash.encodeHex(),
      ),
    )

    val getTxStatusRequestHandlerV1 = GetTransactionExclusionStatusRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val expectedResult = JsonObject()
      .put("txHash", defaultRejectedTransaction.transactionInfo.hash.encodeHex())
      .put("from", defaultRejectedTransaction.transactionInfo.from.encodeHex())
      .put("nonce", defaultRejectedTransaction.transactionInfo.nonce.toHexString())
      .put("txRejectionStage", defaultRejectedTransaction.txRejectionStage.name)
      .put("reasonMessage", defaultRejectedTransaction.reasonMessage)
      .put("timestamp", defaultRejectedTransaction.timestamp.toString())
      .put("blockNumber", defaultRejectedTransaction.blockNumber!!.toHexString())
      .let {
        JsonRpcSuccessResponse(request.id, it)
      }

    val result = getTxStatusRequestHandlerV1.invoke(
      user = null,
      request = request,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(expectedResult, result.get())
  }

  @Test
  fun getTransactionExclusionStatusRequestHandlerV1_invoke_return_null_result() {
    whenever(transactionExclusionServiceMock.getTransactionExclusionStatus(any()))
      .thenReturn(SafeFuture.completedFuture(Ok(null)))

    val request = JsonRpcRequestListParams(
      "2.0",
      "1",
      "linea_getTransactionExclusionStatusV1",
      listOf(
        defaultRejectedTransaction.transactionInfo.hash.encodeHex(),
      ),
    )

    val getTxStatusRequestHandlerV1 = GetTransactionExclusionStatusRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val expectedResult = JsonRpcSuccessResponse(request.id, null)

    val result = getTxStatusRequestHandlerV1.invoke(
      user = null,
      request = request,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(expectedResult, result.get())
  }

  @Test
  fun getTransactionExclusionStatusRequestHandlerV1_invoke_return_failure_result() {
    whenever(transactionExclusionServiceMock.getTransactionExclusionStatus(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          Err(
            TransactionExclusionError(
              ErrorType.SERVER_ERROR,
              "error for unit test",
            ),
          ),
        ),
      )

    val request = JsonRpcRequestListParams(
      "2.0",
      "1",
      "linea_getTransactionExclusionStatusV1",
      listOf(
        defaultRejectedTransaction.transactionInfo.hash.encodeHex(),
      ),
    )

    val getTxStatusRequestHandlerV1 = GetTransactionExclusionStatusRequestHandlerV1(
      transactionExclusionServiceMock,
    )

    val expectedResult = JsonRpcErrorResponse(
      request.id,
      jsonRpcError(
        TransactionExclusionError(
          ErrorType.SERVER_ERROR,
          "error for unit test",
        ),
      ),
    )

    val result = getTxStatusRequestHandlerV1.invoke(
      user = null,
      request = request,
      requestJson = JsonObject(),
    ).get()

    Assertions.assertEquals(expectedResult, result.getError())
  }
}
