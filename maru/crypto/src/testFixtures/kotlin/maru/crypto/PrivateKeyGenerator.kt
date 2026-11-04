/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import io.libp2p.core.PeerId
import io.libp2p.core.crypto.KeyType
import io.libp2p.core.crypto.generateKeyPair
import io.libp2p.core.crypto.unmarshalPrivateKey
import io.libp2p.crypto.keys.unmarshalSecp256k1PrivateKey
import linea.kotlin.encodeHex
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory
import org.hyperledger.besu.cryptoservices.KeyPairSecurityModule
import org.hyperledger.besu.cryptoservices.NodeKey
import tech.pegasys.teku.networking.p2p.libp2p.LibP2PNodeId

/**
 * Utility tool to generate a prefixed private key and corresponding node ID
 */
object PrivateKeyGenerator {
  private val signatureAlgorithm = SignatureAlgorithmFactory.getInstance()

  data class KeyData(
    val privateKey: ByteArray,
    val prefixedPrivateKey: ByteArray,
    val address: ByteArray,
    val peerId: LibP2PNodeId,
    val nodeKey: NodeKey,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as KeyData

      if (!privateKey.contentEquals(other.privateKey)) return false
      if (!prefixedPrivateKey.contentEquals(other.prefixedPrivateKey)) return false
      if (!address.contentEquals(other.address)) return false
      if (peerId != other.peerId) return false
      if (nodeKey.publicKey != other.nodeKey.publicKey) return false

      return true
    }

    override fun hashCode(): Int {
      var result = privateKey.contentHashCode()
      result = 31 * result + prefixedPrivateKey.contentHashCode()
      result = 31 * result + address.contentHashCode()
      result = 31 * result + peerId.hashCode()
      result = 31 * result + nodeKey.hashCode()
      return result
    }

    override fun toString(): String =
      "KeyData(" +
        "privateKey=${privateKey.contentToString()}, " +
        "prefixedPrivateKey=${prefixedPrivateKey.contentToString()}, " +
        "address=${address.contentToString()}, " +
        "peerId=$peerId, " +
        "nodeKey=$nodeKey)"
  }

  fun logKeyData(keyData: KeyData) {
    println(
      "Generated key: " +
        "prefixedPrivateKey=${keyData.prefixedPrivateKey.encodeHex()} " +
        "privateKey=${keyData.privateKey.encodeHex()} " +
        "ethAddress=${keyData.address.encodeHex()} " +
        "libP2pNodeId=${keyData.peerId}",
    )
  }

  @Synchronized
  fun getKeyData(privateKey: ByteArray): KeyData {
    // Sometimes keyPair has 1 byte more so we just take the last 32 bytes ¯\_(ツ)_/¯
    val normalizedPrivateKey = ensureKeyIsExactly32BytesLong(privateKey)
    // The curve parameters for the block signing are different from what LibP2P is using
    val signingAddress = SecpCrypto.privateKeyToAddress(normalizedPrivateKey)
    val privateKeyTyped = unmarshalSecp256k1PrivateKey(normalizedPrivateKey)
    val peerId = PeerId.fromPubKey(privateKeyTyped.publicKey())
    val libP2PNodeId = LibP2PNodeId(peerId)
    val nodeKey = createNodeKey(normalizedPrivateKey)
    val keyData =
      KeyData(
        privateKey = privateKeyTyped.raw(),
        // marshalPrivateKey(privateKeyTyped) round-trips through BigInteger, which drops leading
        // zero bytes (~0.4% of random keys): the protobuf payload then has 31 bytes and length byte
        // 0x1F, and readers stripping a fixed 4-byte prefix via takeLast(32) — see SecpCrypto.
        // privateKeyBytesWithoutPrefix — end up with a corrupted key (the length byte leaks into
        // the payload).
        // Build the libp2p protobuf wrapper manually so the payload is always exactly 32 bytes:
        // field 1 (type) = varint SECP256K1 (2), field 2 (data) = length-delimited 32-byte key.
        prefixedPrivateKey = byteArrayOf(0x08, 0x02, 0x12, 0x20) + normalizedPrivateKey,
        address = signingAddress,
        peerId = libP2PNodeId,
        nodeKey = nodeKey,
      )
    // Self-consistency invariant: the takeLast(32) readers above must recover the same key.
    // Guards against any future change reintroducing a length-sensitive prefix/payload mismatch.
    val addressFromPrefixedKey =
      SecpCrypto.privateKeyToAddress(SecpCrypto.privateKeyBytesWithoutPrefix(keyData.prefixedPrivateKey))
    require(keyData.address.contentEquals(addressFromPrefixedKey)) {
      "Torn KeyData detected: address=${keyData.address.encodeHex(false)} but " +
        "prefixedPrivateKey maps to address=${addressFromPrefixedKey.encodeHex(false)}"
    }
    return keyData
  }

  private fun ensureKeyIsExactly32BytesLong(privateKey: ByteArray): ByteArray =
    when {
      privateKey.size == 32 -> privateKey
      privateKey.size < 32 -> ByteArray(32 - privateKey.size) + privateKey
      else -> privateKey.takeLast(32).toByteArray()
    }

  private fun createNodeKey(normalizedPrivateKey: ByteArray): NodeKey {
    val besuPrivateKey = signatureAlgorithm.createPrivateKey(Bytes32.wrap(normalizedPrivateKey))
    val keyPair = signatureAlgorithm.createKeyPair(besuPrivateKey)
    val keyPairSecurityModule = KeyPairSecurityModule(keyPair)
    return NodeKey(keyPairSecurityModule)
  }

  @Synchronized
  fun getKeyDataByPrefixedKey(prefixedPrivateKey: ByteArray): KeyData {
    val privateKeyTyped = unmarshalPrivateKey(prefixedPrivateKey)
    return getKeyData(privateKeyTyped.raw())
  }

  @Synchronized
  fun generatePrivateKey(): KeyData {
    val (privKey, pubKey) = generateKeyPair(KeyType.SECP256K1)
    return getKeyData(privKey.raw())
  }
}
