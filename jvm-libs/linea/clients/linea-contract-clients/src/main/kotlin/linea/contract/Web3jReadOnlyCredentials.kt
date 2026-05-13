package linea.contract

import linea.kotlin.encodeHex
import org.web3j.crypto.Credentials

/**
 * Zero-key credentials used for read-only Web3j contract calls.
 * Web3j contract.load() requires non-null Credentials even for view-only calls;
 * this constant satisfies that requirement without exposing a real private key.
 */
internal val FAKE_READ_ONLY_CREDENTIALS: Credentials = Credentials.create(ByteArray(32).encodeHex())
