package credentialauth

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

const (
	sliderChallengeProofXTolerance = 3
	sliderChallengeProofYTolerance = 0
	challengeProofDigestSetPrefix  = "v1:challenge-proof-digest-set:"
)

func acceptedChallengeProofs(kind ChallengeKind, proof string) ([]string, error) {
	kind = ChallengeKind(strings.TrimSpace(string(kind)))
	proof = strings.TrimSpace(proof)
	if kind != ChallengeKindSlider {
		if proof == "" {
			return nil, fmt.Errorf("%w: challenge proof is required", ErrInvalidInput)
		}
		return []string{proof}, nil
	}
	x, y, err := parseSlideChallengeProof(proof)
	if err != nil {
		return nil, err
	}
	proofs := make([]string, 0, (sliderChallengeProofXTolerance*2+1)*(sliderChallengeProofYTolerance*2+1))
	for nextY := y - sliderChallengeProofYTolerance; nextY <= y+sliderChallengeProofYTolerance; nextY++ {
		if nextY < 0 {
			continue
		}
		for nextX := x - sliderChallengeProofXTolerance; nextX <= x+sliderChallengeProofXTolerance; nextX++ {
			if nextX < 0 {
				continue
			}
			proofs = append(proofs, FormatSlideChallengeProof(nextX, nextY))
		}
	}
	if len(proofs) == 0 {
		return nil, fmt.Errorf("%w: slide challenge proof is invalid", ErrInvalidInput)
	}
	return proofs, nil
}

func normalizeChallengeProof(kind ChallengeKind, proof string) (string, error) {
	proofs, err := acceptedChallengeProofs(kind, proof)
	if err != nil {
		return "", err
	}
	if ChallengeKind(strings.TrimSpace(string(kind))) != ChallengeKindSlider {
		return proofs[0], nil
	}
	x, y, err := parseSlideChallengeProof(proof)
	if err != nil {
		return "", err
	}
	return FormatSlideChallengeProof(x, y), nil
}

func parseSlideChallengeProof(proof string) (int, int, error) {
	proof = strings.TrimSpace(proof)
	if proof == "" {
		return 0, 0, fmt.Errorf("%w: slide challenge proof is required", ErrInvalidInput)
	}
	if strings.HasPrefix(proof, "{") {
		if x, y, ok := parseSlideChallengeProofJSON(proof); ok {
			return x, y, nil
		}
	}
	if x, y, ok := parseSlideChallengeProofPairs(proof); ok {
		return x, y, nil
	}
	return 0, 0, fmt.Errorf("%w: slide challenge proof format is invalid", ErrInvalidInput)
}

func parseSlideChallengeProofJSON(proof string) (int, int, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(proof), &raw); err != nil {
		return 0, 0, false
	}
	x, ok := slideCoordinateFromJSON(raw["x"])
	if !ok {
		return 0, 0, false
	}
	y, ok := slideCoordinateFromJSON(raw["y"])
	if !ok {
		return 0, 0, false
	}
	return x, y, true
}

func slideCoordinateFromJSON(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return parseSlideCoordinate(text)
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return parseSlideCoordinate(number.String())
	}
	return 0, false
}

func parseSlideChallengeProofPairs(proof string) (int, int, bool) {
	values := map[string]string{}
	for _, part := range strings.FieldsFunc(proof, func(value rune) bool {
		return value == '&' || value == ';' || value == ','
	}) {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if decoded, err := url.QueryUnescape(value); err == nil {
			value = decoded
		}
		if key == "x" || key == "y" {
			values[key] = value
		}
	}
	x, ok := parseSlideCoordinate(values["x"])
	if !ok {
		return 0, 0, false
	}
	y, ok := parseSlideCoordinate(values["y"])
	if !ok {
		return 0, 0, false
	}
	return x, y, true
}

func parseSlideCoordinate(value string) (int, bool) {
	coordinate, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(coordinate) || math.IsInf(coordinate, 0) || coordinate < 0 || coordinate > 100000 {
		return 0, false
	}
	return int(math.Round(coordinate)), true
}
