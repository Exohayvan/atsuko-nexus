package types

import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/hex"
    "fmt"

    "atsuko-nexus/src/logger"
)

// generateKeyPairHex returns the private+public key as hex strings.
func generateKeyPairHex() (privHex, pubHex string, err error) {
    pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return "", "", err
    }
    return hex.EncodeToString(priv), hex.EncodeToString(pub), nil
}

// Henley is your “keygen” function — it generates a new keypair and logs it.
func GenKey() {
    priv, pub, err := generateKeyPairHex()
    if err != nil {
        logger.Log("ERROR", "GENKEY", "key generation failed: "+err.Error())
        return
    }

    // you can split into two logs, or combine into one
    logger.Log("INFO", "GENKEY", fmt.Sprintf("Private Key (hex): %s", priv))
    logger.Log("INFO", "GENKEY", fmt.Sprintf("Public Key  (hex): %s", pub))
}
