package net.consensys.zkevm.ethereum.submission

import org.apache.logging.log4j.Logger
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.mockito.Mockito.mock
import org.mockito.kotlin.eq
import org.mockito.kotlin.verify

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class LoggingHelperTest {
  private lateinit var logger: Logger

  private val blobSubmissionFailedLogMsg = "{} for blob submission failed: blob={} errorMessage={}"
  private val blobSubmissionFailedIntervalStr = "[4025752..4031499]5748"
  private val insufficientMaxFeePerBlobGasErrMsg = "org.web3j.tx.exceptions.ContractCallException:" +
    " Contract Call has been reverted by the EVM with the reason:" +
    " 'err: max fee per blob gas less than block blob gas fee:" +
    " address 0x47C63d1E391FcB3dCdC40C4d7fA58ADb172f8c38 blobGasFeeCap: 1875810596," +
    " blobBaseFee: 1962046498 (supplied gas 1000000)'. revertReason=UNKNOWN errorData=null"

  private val insufficientMaxFeePerGasErrMsg = "org.web3j.tx.exceptions.ContractCallException:" +
    " Contract Call has been reverted by the EVM with the reason:" +
    " 'err: max fee per gas less than block base fee:" +
    " address 0x47C63d1E391FcB3dCdC40C4d7fA58ADb172f8c38, maxFeePerGas: 300000000000," +
    " baseFee: 302246075616 (supplied gas 1000000)'. revertReason=UNKNOWN errorData=null"

  private val unknownErrMsg = "org.web3j.tx.exceptions.ContractCallException:" +
    " Contract Call has been reverted by the EVM with the reason:" +
    " 'err: unknown error:" +
    " address 0x47C63d1E391FcB3dCdC40C4d7fA58ADb172f8c38'. revertReason=UNKNOWN errorData=null"

  @BeforeEach
  fun setUp() {
    logger = mock()
  }

  @Test
  fun `insufficient max fee per gas with isEthCall is true triggers rewrite info message`() {
    val error = RuntimeException(insufficientMaxFeePerGasErrMsg)
    logSubmissionError(
      log = logger,
      logMessage = blobSubmissionFailedLogMsg,
      intervalString = blobSubmissionFailedIntervalStr,
      error = error,
      isEthCall = true,
    )
    val expectedErrorMessage =
      "maxFeePerGas less than block base fee:" +
        " address 0x47C63d1E391FcB3dCdC40C4d7fA58ADb172f8c38, maxFeePerGas: 300000000000," +
        " baseFee: 302246075616 (supplied gas 1000000)"

    verify(logger).info(
      eq(blobSubmissionFailedLogMsg),
      eq("eth_call"),
      eq(blobSubmissionFailedIntervalStr),
      eq(expectedErrorMessage),
    )
  }

  @Test
  fun `insufficient max fee per blob gas with isEthCall is true triggers rewrite info message`() {
    val error = RuntimeException(insufficientMaxFeePerBlobGasErrMsg)
    logSubmissionError(
      log = logger,
      logMessage = blobSubmissionFailedLogMsg,
      intervalString = blobSubmissionFailedIntervalStr,
      error = error,
      isEthCall = true,
    )
    val expectedErrorMessage =
      "maxFeePerBlobGas less than block blob gas fee:" +
        " address 0x47C63d1E391FcB3dCdC40C4d7fA58ADb172f8c38 maxFeePerBlobGas: 1875810596," +
        " blobBaseFee: 1962046498 (supplied gas 1000000)"

    verify(logger).info(
      eq(blobSubmissionFailedLogMsg),
      eq("eth_call"),
      eq(blobSubmissionFailedIntervalStr),
      eq(expectedErrorMessage),
    )
  }

  @Test
  fun `insufficient max fee per gas with isEthCall is false do not trigger rewrite error message`() {
    val error = RuntimeException(insufficientMaxFeePerGasErrMsg)
    logSubmissionError(
      log = logger,
      logMessage = blobSubmissionFailedLogMsg,
      intervalString = blobSubmissionFailedIntervalStr,
      error = error,
      isEthCall = false,
    )

    verify(logger).error(
      eq(blobSubmissionFailedLogMsg),
      eq("eth_sendRawTransaction"),
      eq(blobSubmissionFailedIntervalStr),
      eq(insufficientMaxFeePerGasErrMsg),
      eq(error),
    )
  }

  @Test
  fun `insufficient max fee per blob gas with isEthCall is false do not trigger rewrite error message`() {
    val error = RuntimeException(insufficientMaxFeePerBlobGasErrMsg)
    logSubmissionError(
      log = logger,
      logMessage = blobSubmissionFailedLogMsg,
      intervalString = blobSubmissionFailedIntervalStr,
      error = error,
      isEthCall = false,
    )

    verify(logger).error(
      eq(blobSubmissionFailedLogMsg),
      eq("eth_sendRawTransaction"),
      eq(blobSubmissionFailedIntervalStr),
      eq(insufficientMaxFeePerBlobGasErrMsg),
      eq(error),
    )
  }

  @Test
  fun `Other error with isEthCall is true do not trigger rewrite error message`() {
    val error = RuntimeException(unknownErrMsg)
    logSubmissionError(
      log = logger,
      logMessage = blobSubmissionFailedLogMsg,
      intervalString = blobSubmissionFailedIntervalStr,
      error = error,
      isEthCall = true,
    )

    verify(logger).error(
      eq(blobSubmissionFailedLogMsg),
      eq("eth_call"),
      eq(blobSubmissionFailedIntervalStr),
      eq(unknownErrMsg),
      eq(error),
    )
  }
}
