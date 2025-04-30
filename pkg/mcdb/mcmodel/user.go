package mcmodel

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

type User struct {
	ID         int    `json:"id"`
	UUID       string `json:"uuid"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	GlobusUser string `json:"globus_user"`
	ApiToken   string `json:"-"`
	Password   string `json:"-"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type PayloadStructure struct {
	IV    string `json:"iv"`
	Value string `json:"value"`
	Mac   string `json:"mac"`
}

// DecryptedApiToken returns the decrypted API token
func (u *User) DecryptedApiToken(key []byte) (string, error) {
	if u.ApiToken == "" {
		return "", errors.New("api token is empty")
	}

	// Get the app key from .env file and then do:
	// appKey = strings.TrimPrefix(appKey, "base64:")

	// Decode the base64 encoded string
	encrypted, err := base64.StdEncoding.DecodeString(u.ApiToken)
	if err != nil {
		return "", err
	}

	// The payload should be at least having a MAC (32 bytes) and IV (16 bytes)
	if len(encrypted) < 48 {
		return "", errors.New("invalid payload length")
	}

	// Extract the IV and ciphertext
	iv := encrypted[:aes.BlockSize]
	ciphertext := encrypted[aes.BlockSize:]

	// Create a cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create decrypter
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Unpad the plaintext
	unpadded, err := pkcs7Unpad(plaintext)
	if err != nil {
		return "", err
	}

	// Parse the JSON payload
	var payload PayloadStructure

	if err := json.Unmarshal(unpadded, &payload); err != nil {
		return "", err
	}

	return payload.Value, nil
}

func ValidateMAC(payload PayloadStructure, key []byte) bool {
	hash := hmac.New(sha256.New, key)
	toHash := payload.IV + payload.Value
	hash.Write([]byte(toHash))
	calculatedMac := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	return hmac.Equal([]byte(calculatedMac), []byte(payload.Mac))
}

// pkcs7Unpad removes PKCS#7 padding
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("invalid padding")
	}

	padding := int(data[len(data)-1])
	if padding > len(data) {
		return nil, errors.New("invalid padding size")
	}

	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}

// Get the app key from .env file and then do:
// appKey = strings.TrimPrefix(appKey, "base64:")

// func main() {
//    user := &mcmodel.User{
//        ApiToken: "encrypted_token_here", // Base64 encoded encrypted token
//    }
//
//    Get the app key from .env file and then do:
//    appKey = strings.TrimPrefix(appKey, "base64:")
//    decrypted, err := user.DecryptedApiToken(appKey)
//    if err != nil {
//        log.Fatal(err)
//    }
//    fmt.Println("Decrypted token:", decrypted)
//}
