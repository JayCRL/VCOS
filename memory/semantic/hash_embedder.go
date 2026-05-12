package semantic

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strings"
	"unicode"
)

// HashEmbedder produces deterministic, dependency-free embeddings via the
// feature-hashing trick: split text into tokens, hash each token to a bucket,
// accumulate signed counts, then L2-normalize. Result is stable across runs
// without any ML model. Good enough to bootstrap MOE / dashboard search; not
// a substitute for a real embedding model in production retrieval quality.
type HashEmbedder struct {
	// Dimension is the vector size. Defaults to 256 when zero.
	Dimension int
}

// NewHashEmbedder returns a HashEmbedder with sensible defaults.
func NewHashEmbedder() *HashEmbedder { return &HashEmbedder{Dimension: 256} }

func (h *HashEmbedder) Dim() int {
	if h.Dimension <= 0 {
		return 256
	}
	return h.Dimension
}

func (h *HashEmbedder) Embed(text string) ([]float32, error) {
	dim := h.Dim()
	vec := make([]float32, dim)
	if text == "" {
		return vec, nil
	}
	for _, tok := range tokenize(text) {
		bucket, sign := bucketSign(tok, dim)
		vec[bucket] += sign
	}
	// L2-normalize so vectors live on the unit hypersphere — makes dot
	// product equivalent to cosine similarity.
	var sumSq float64
	for _, v := range vec {
		sumSq += float64(v) * float64(v)
	}
	if sumSq > 0 {
		inv := float32(1.0 / math.Sqrt(sumSq))
		for i := range vec {
			vec[i] *= inv
		}
	}
	return vec, nil
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := fields[:0]
	for _, f := range fields {
		if len(f) >= 2 {
			out = append(out, f)
		}
	}
	return out
}

func bucketSign(token string, dim int) (int, float32) {
	sum := sha256.Sum256([]byte(token))
	bucket := int(binary.BigEndian.Uint32(sum[:4])) % dim
	if bucket < 0 {
		bucket += dim
	}
	if sum[4]&0x01 == 0 {
		return bucket, 1
	}
	return bucket, -1
}

// CosineSimilarity returns dot-product similarity between two equally-sized
// vectors. Inputs are assumed normalized; for un-normalized inputs the caller
// should divide by the L2 norms.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
