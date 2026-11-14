package dev.clipconnect.app.network

/**
 * Bounded validation for host addresses and ports used before connecting.
 * Pure logic, unit-testable without a network.
 */
object NetValidation {

    /** @return true if [addr] is a dotted-quad IPv4 or a non-empty hostname. */
    fun isValidAddress(addr: String): Boolean {
        if (addr.isBlank()) return false
        val parts = addr.split(".")
        if (parts.size == 4) {
            // IPv4 dotted quad.
            for (part in parts) {
                if (part.isEmpty() || !part.all { it.isDigit() }) return false
                val num = part.toIntOrNull() ?: return false
                if (num < 0 || num > 255) return false
            }
            return true
        }
        // Anything with all-numeric dotted parts that isn't a 4-part quad is a
        // malformed IPv4 and must be rejected.
        val allNumeric = parts.all { it.isNotEmpty() && it.all { c -> c.isDigit() } }
        if (allNumeric && parts.size != 4) return false
        // Otherwise treat as a hostname: allow alnum, dots, hyphens, no spaces.
        return addr.all { it.isLetterOrDigit() || it == '.' || it == '-' }
    }

    fun isValidPort(port: Int): Boolean = port in 1..65535
}
