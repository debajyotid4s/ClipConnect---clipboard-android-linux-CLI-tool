package dev.clipconnect.app.network

import dev.clipconnect.app.crypto.Crypto
import dev.clipconnect.app.crypto.FrameCodec
import android.util.Log
import org.json.JSONObject
import java.io.IOException
import java.security.SecureRandom

/**
 * Perform the CCN1 protocol handshake:
 * 1. Client sends CCN1 + 16-byte client nonce
 * 2. Server sends 16-byte server nonce
 * 3. Both sides derive c2s/s2c keys via HKDF-SHA256
 * 4. Exchange hello frames
 */
internal fun TcpClient.performHandshake(secret: ByteArray) {
    val out = socketOut!!
    val input = socketIn!!

    // Generate client nonce
    val clientNonce = ByteArray(NONCE_LEN)
    SecureRandom.getInstanceStrong().nextBytes(clientNonce)

    // Step 1: Send CCN1 + clientNonce
    out.write(MAGIC.toByteArray(Charsets.US_ASCII))
    out.write(clientNonce)
    out.flush()
    Log.d(TAG, "Sent CCN1 + client nonce")

    // Step 2: Read server nonce
    val serverNonce = ByteArray(NONCE_LEN)
    input.readFully(serverNonce)
    Log.d(TAG, "Received server nonce")

    // Step 3: Derive keys using HKDF-SHA256
    val salt = clientNonce + serverNonce
    val keyC2S = Crypto.hkdfSha256(secret, salt, INFO_C2S, KEY_LEN)
    val keyS2C = Crypto.hkdfSha256(secret, salt, INFO_S2C, KEY_LEN)

    c2sCodec = FrameCodec(keyC2S)
    s2cCodec = FrameCodec(keyS2C)

    // Step 4: Exchange hello frames. The server mints a fresh per-connection
    // id ("cid") and announces it in its hello; we adopt it and echo it back
    // so both sides key the durable outbox on that value. A fresh id is minted
    // on every connection, so a reconnected phone cannot claim another phone's
    // queued clips. We also send our stable install id ("pid") as a legacy hint.
    val serverHello = s2cCodec!!.readFrame(input)
    val helloResponse = String(serverHello, Charsets.UTF_8)
    Log.d(TAG, "Server hello: $helloResponse")
    val serverHelloObj = JSONObject(helloResponse)
    val serverCid = runCatching { serverHelloObj.optString("cid") }.getOrDefault("")
    if (serverCid.isBlank()) {
        throw IOException("Server hello missing cid: $helloResponse")
    }
    connectionId = serverCid
    val helloObj = JSONObject().apply {
        put("t", "hello")
        put("v", 1)
        if (serverCid.isNotBlank()) put("cid", serverCid)
        if (phoneId.isNotBlank()) put("pid", phoneId)
    }
    val hello = helloObj.toString().toByteArray(Charsets.UTF_8)
    c2sCodec!!.writeFrame(out, hello)
    out.flush()
    Log.d(TAG, "Sent client hello (echoed server cid)")
}
