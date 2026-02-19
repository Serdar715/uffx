package input

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/url"
	"strings"
)

// Encoder is the interface that all encoders must implement
type Encoder interface {
	Encode(data []byte) ([]byte, error)
}

// Chain holds a sequence of encoders
type Chain struct {
	encoders []Encoder
}

// NewChain creates a new empty chain
func NewChain() *Chain {
	return &Chain{
		encoders: make([]Encoder, 0),
	}
}

// Initialize parses the encoder names and builds the chain
func (c *Chain) Initialize(names []string) error {
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}

		var enc Encoder
		switch name {
		case "md5":
			enc = &MD5Encoder{}
		case "sha1":
			enc = &SHA1Encoder{}
		case "sha256":
			enc = &SHA256Encoder{}
		case "sha512":
			enc = &SHA512Encoder{}
		case "base64":
			enc = &Base64Encoder{}
		case "hex":
			enc = &HexEncoder{}
		case "url":
			enc = &URLEncoder{}
		case "html":
			enc = &HTMLEncoder{}
		case "doubleurl":
			enc = &DoubleURLEncoder{}
		case "uppercase":
			enc = &UppercaseEncoder{}
		case "lowercase":
			enc = &LowercaseEncoder{}
		default:
			return fmt.Errorf("unknown encoder: %s", name)
		}
		c.encoders = append(c.encoders, enc)
	}
	return nil
}

// Encode runs the data through the chain of encoders
func (c *Chain) Encode(data []byte) ([]byte, error) {
	var err error
	current := data
	for _, enc := range c.encoders {
		current, err = enc.Encode(current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// --- Encoder Implementations ---

type MD5Encoder struct{}

func (e *MD5Encoder) Encode(data []byte) ([]byte, error) {
	hash := md5.Sum(data)
	return []byte(hex.EncodeToString(hash[:])), nil
}

type SHA1Encoder struct{}

func (e *SHA1Encoder) Encode(data []byte) ([]byte, error) {
	hash := sha1.Sum(data)
	return []byte(hex.EncodeToString(hash[:])), nil
}

type SHA256Encoder struct{}

func (e *SHA256Encoder) Encode(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	return []byte(hex.EncodeToString(hash[:])), nil
}

type SHA512Encoder struct{}

func (e *SHA512Encoder) Encode(data []byte) ([]byte, error) {
	hash := sha512.Sum512(data)
	return []byte(hex.EncodeToString(hash[:])), nil
}

type Base64Encoder struct{}

func (e *Base64Encoder) Encode(data []byte) ([]byte, error) {
	return []byte(base64.StdEncoding.EncodeToString(data)), nil
}

type HexEncoder struct{}

func (e *HexEncoder) Encode(data []byte) ([]byte, error) {
	return []byte(hex.EncodeToString(data)), nil
}

type URLEncoder struct{}

func (e *URLEncoder) Encode(data []byte) ([]byte, error) {
	return []byte(url.QueryEscape(string(data))), nil
}

type DoubleURLEncoder struct{}

func (e *DoubleURLEncoder) Encode(data []byte) ([]byte, error) {
	first := url.QueryEscape(string(data))
	return []byte(url.QueryEscape(first)), nil
}

type HTMLEncoder struct{}

func (e *HTMLEncoder) Encode(data []byte) ([]byte, error) {
	return []byte(html.EscapeString(string(data))), nil
}

type UppercaseEncoder struct{}

func (e *UppercaseEncoder) Encode(data []byte) ([]byte, error) {
	return []byte(strings.ToUpper(string(data))), nil
}

type LowercaseEncoder struct{}

func (e *LowercaseEncoder) Encode(data []byte) ([]byte, error) {
	return []byte(strings.ToLower(string(data))), nil
}
