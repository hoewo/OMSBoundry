package referenceoa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"omsbizfoundry/bonus-distribution/internal/bonus"
)

const (
	PolicyKey     = "reference-oa-bonus-v1"
	PolicyVersion = "1.0.0"
)

type Evaluator struct{}

func (Evaluator) Evaluate(_ context.Context, bonusType string, baseMinor int64, currency string, request bonus.PolicyRequest) (bonus.PolicySnapshot, int64, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if (bonusType != bonus.TypeBusiness && bonusType != bonus.TypeProduction) || baseMinor <= 0 || request.Key != PolicyKey || request.Version != PolicyVersion || request.RateBasisPoints <= 0 || request.RateBasisPoints > 10000 || request.Rounding != bonus.RoundingHalfUp {
		return bonus.PolicySnapshot{}, 0, bonus.ErrPolicyInvalid
	}
	// Exact semantics: base × (rate basis points / 10000) ÷ 1.065.
	// 1.065 is represented as 1065/1000; no binary floating point is used.
	numerator := new(big.Int).Mul(big.NewInt(baseMinor), big.NewInt(request.RateBasisPoints))
	numerator.Mul(numerator, big.NewInt(1000))
	denominator := big.NewInt(10000 * 1065)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return bonus.PolicySnapshot{}, 0, fmt.Errorf("reference OA bonus amount overflow")
	}
	canonical := fmt.Sprintf("%s|%s|%s|%d|%s", PolicyKey, PolicyVersion, bonusType, request.RateBasisPoints, request.Rounding)
	digest := sha256.Sum256([]byte(canonical))
	snapshot := bonus.PolicySnapshot{
		Key:              PolicyKey,
		Version:          PolicyVersion,
		BonusType:        bonusType,
		RateBasisPoints:  request.RateBasisPoints,
		Rounding:         bonus.RoundingHalfUp,
		Formula:          "base_minor * rate_basis_points / 10000 / 1.065",
		ParametersDigest: "sha256:" + hex.EncodeToString(digest[:]),
	}
	return snapshot, quotient.Int64(), nil
}
