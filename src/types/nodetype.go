package types

import (
    "bytes"
    "crypto/ed25519"
    "encoding/hex"
    "fmt"

    "atsuko-nexus/src/logger"
    "atsuko-nexus/src/settings"
)

// expectedAdminPubKeyHex is the reference public key for admin verification.
const expectedAdminPubKeyHex = "126b187c2410505fe5cba6259de4bd15d1567fd0e6559514f91911e1887a0d56"

// NodeType checks the stored identity.admin_key setting against the expected admin public key.
// It derives the public key from the given private key (hex), compares it, logs the result, and returns "admin" or "default".
func NodeType() string {
    raw := settings.Get("identity.admin_key")
    keyHex, ok := raw.(string)
    if !ok {
        logger.Log("WARN", "NODETYPE", "identity.admin_key not set or invalid type")
        return "default"
    }

    privBytes, err := hex.DecodeString(keyHex)
    if err != nil {
        logger.Log("ERROR", "NODETYPE", fmt.Sprintf("failed to decode admin_key hex: %v", err))
        return "default"
    }

    var pubKey ed25519.PublicKey
    switch len(privBytes) {
    case ed25519.SeedSize:
        privKey := ed25519.NewKeyFromSeed(privBytes)
        pubKey = privKey.Public().(ed25519.PublicKey)
    case ed25519.PrivateKeySize:
        privKey := ed25519.PrivateKey(privBytes)
        pubKey = privKey.Public().(ed25519.PublicKey)
    default:
        logger.Log("ERROR", "NODETYPE", fmt.Sprintf("invalid private key length: %d", len(privBytes)))
        return "default"
    }

    expectedPubBytes, err := hex.DecodeString(expectedAdminPubKeyHex)
    if err != nil {
        logger.Log("ERROR", "NODETYPE", fmt.Sprintf("failed to decode expected public key hex: %v", err))
        return "default"
    }

    role := "default"
    if bytes.Equal(pubKey, expectedPubBytes) {
        role = "admin"
    }

    logger.Log("DEBUG", "NODETYPE", fmt.Sprintf("Determined node role: %s", role))
    return role
}