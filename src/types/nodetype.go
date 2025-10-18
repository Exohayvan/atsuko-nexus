package types

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/settings"
)

// expectedAdminPubKeyHex is the reference public key for admin verification.
const expectedAdminPubKeyHex = "126b187c2410505fe5cba6259de4bd15d1567fd0e6559514f91911e1887a0d56"

// NodeType resolves the node's declared role based on the configured admin key.
// When a valid hex-encoded Ed25519 private key is supplied, the derived public key is compared
// against the expected admin key. Otherwise, the plaintext value is used directly as the role.
func NodeType() string {
	raw := settings.Get("identity.admin_key")
	keyValue, ok := raw.(string)
	if !ok {
		logger.Log("WARN", "NODETYPE", "identity.admin_key not set or invalid type")
		return "default"
	}

	keyValue = strings.TrimSpace(keyValue)
	if keyValue == "" {
		logger.Log("DEBUG", "NODETYPE", "identity.admin_key empty; defaulting to standard role")
		return "default"
	}

	if privBytes, err := hex.DecodeString(keyValue); err == nil {
		if role := roleFromPrivateKey(privBytes); role != "" {
			logger.Log("DEBUG", "NODETYPE", fmt.Sprintf("Resolved node role from private key: %s", role))
			return role
		}
		logger.Log("WARN", "NODETYPE", "Admin private key decoded but did not match expected admin identity; using default role")
		return "default"
	} else {
		logger.Log("WARN", "NODETYPE", fmt.Sprintf("identity.admin_key is not valid hex (%v); treating as plaintext", err))
		return normalizeRole(keyValue)
	}
}

func roleFromPrivateKey(privBytes []byte) string {
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
		return ""
	}

	expectedPubBytes, err := hex.DecodeString(expectedAdminPubKeyHex)
	if err != nil {
		logger.Log("ERROR", "NODETYPE", fmt.Sprintf("failed to decode expected public key hex: %v", err))
		return ""
	}

	if bytes.Equal(pubKey, expectedPubBytes) {
		return "admin"
	}
	return ""
}

func normalizeRole(raw string) string {
	role := strings.ToLower(strings.TrimSpace(raw))
	switch role {
	case "", "none", "default":
		return "default"
	case "admin":
		logger.Log("INFO", "NODETYPE", "Plaintext admin role detected")
		return "admin"
	default:
		logger.Log("DEBUG", "NODETYPE", fmt.Sprintf("Using plaintext node role: %s", role))
		return role
	}
}
