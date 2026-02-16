package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Signer handles RSA-PSS request signing for Kalshi API authentication.
type Signer struct {
	keyID      string
	privateKey *rsa.PrivateKey
}

// NewSigner creates a new Signer by loading the RSA private key from a PEM file.
func NewSigner(keyID, privateKeyPath string) (*Signer, error) {
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	privateKey, err := parsePEMKey(keyData)
	if err != nil {
		return nil, err
	}

	return &Signer{
		keyID:      keyID,
		privateKey: privateKey,
	}, nil
}

// NewSignerFromBytes creates a Signer from PEM-encoded RSA private key bytes.
// Used when the key is stored in the database rather than on disk.
func NewSignerFromBytes(keyID string, pemData []byte) (*Signer, error) {
	privateKey, err := parsePEMKey(pemData)
	if err != nil {
		return nil, err
	}
	return &Signer{keyID: keyID, privateKey: privateKey}, nil
}

// parsePEMKey extracts an RSA private key from PEM data (PKCS#8 or PKCS#1).
func parsePEMKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from private key")
	}

	// Try parsing as PKCS#8 first (more common for generated keys)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not an RSA key")
		}
		return rsaKey, nil
	}

	// Fall back to PKCS#1
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key (tried PKCS#8 and PKCS#1): %w", err)
	}

	return rsaKey, nil
}

// SignRequest generates the authentication headers for a Kalshi API request.
// The method should be uppercase (GET, POST, etc.).
// The path should NOT include query parameters.
func (s *Signer) SignRequest(method, path string) (headers map[string]string, err error) {
	// Timestamp in milliseconds
	timestamp := time.Now().UnixMilli()
	timestampStr := strconv.FormatInt(timestamp, 10)

	// Message to sign: timestamp + method + path
	message := timestampStr + method + path

	// Hash the message with SHA256
	hash := sha256.Sum256([]byte(message))

	// Sign with RSA-PSS
	// Salt length matches SHA256 digest length (32 bytes)
	signature, err := rsa.SignPSS(rand.Reader, s.privateKey, crypto.SHA256, hash[:], &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Base64 encode the signature
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	return map[string]string{
		"KALSHI-ACCESS-KEY":       s.keyID,
		"KALSHI-ACCESS-TIMESTAMP": timestampStr,
		"KALSHI-ACCESS-SIGNATURE": signatureB64,
	}, nil
}

// KeyID returns the API key ID.
func (s *Signer) KeyID() string {
	return s.keyID
}
