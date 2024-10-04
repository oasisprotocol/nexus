package pvss

import (
	"crypto/elliptic"
	"encoding/base64"
	"fmt"

	"go.dedis.ch/kyber/v3"
)

// As convenient as it is to use kyber's PVSS implementation, scalars and
// points being interfaces makes s11n a huge pain, and mandates using
// wrapper types so that this can play nice with CBOR/JSON etc.
//
// Aut viam inveniam aut faciam.

// Point is an elliptic curve point.
type Point struct {
	inner kyber.Point
}

// Inner returns the actual kyber.Point.
// removed func

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (p *Point) UnmarshalBinary(data []byte) error {
	inner := suite.Point()
	if err := inner.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("pvss/s11n: failed to deserialize point: %w", err)
	}

	checkPoint := Point{inner: inner}
	if err := checkPoint.isWellFormed(); err != nil {
		return fmt.Errorf("pvss/s11n: deserialized point is invalid: %w", err)
	}
	if checkPoint2, ok := inner.(isCanonicalAble); ok {
		// edwards25519.Point.IsCanonical takes a buffer, since points
		// that get serialized are always in canonical form.
		if !checkPoint2.IsCanonical(data) {
			return fmt.Errorf("pvss/s11n: point is not in canonical form")
		}
	}

	p.inner = inner

	return nil
}

// UnmarshalPEM decodes a PEM marshaled point.
// removed func

// UnmarshalText decodes a text marshaled point.
func (p *Point) UnmarshalText(text []byte) error {
	b, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return fmt.Errorf("pvss/s11n: failed to deserialize base64 encoded point: %w", err)
	}

	return p.UnmarshalBinary(b)
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p Point) MarshalBinary() ([]byte, error) {
	if err := p.isWellFormed(); err != nil {
		return nil, fmt.Errorf("pvss/s11n: refusing to serialize invalid point: %w", err)
	}

	data, err := p.inner.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("pvss/s11n: failed to serialize point: %w", err)
	}

	return data, nil
}

// MarshalPEM encodes a point into PEM form.
// removed func

// MarshalText encodes a point into text form.
func (p *Point) MarshalText() ([]byte, error) {
	b, err := p.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return []byte(base64.StdEncoding.EncodeToString(b)), nil
}

// LoadPEM loads a point from a PEM file on disk.  Iff the point is missing
// and a Scalar is provided, the Scalar's corresponding point will be written
// and loaded.
// removed func

func (p *Point) isWellFormed() error {
	// Can never happen(?), but check anyway.
	if p.inner == nil {
		return fmt.Errorf("pvss/s11n: point is missing")
	}

	if !pointIsValid(p.inner) {
		return fmt.Errorf("pvss/s11n: point is invalid")
	}

	return nil
}

type validAble interface {
	Valid() bool
}

type hasSmallOrderAble interface {
	HasSmallOrder() bool
}

type isCanonicalAble interface {
	IsCanonical([]byte) bool
}

func pointIsValid(point kyber.Point) bool {
	switch validator := point.(type) {
	case validAble:
		// P-256 point validation (ensures point is on curve)
		//
		// Note: Kyber's idea of a valid point includes the point at
		// infinity, which does not ensure contributory behavior when
		// doing ECDH.

		// We write out the point to binary data, and unmarshal
		// it with elliptic.Unmarshal, which checks to see if the
		// point is on the curve (while rejecting the point at
		// infinity).
		//
		// In theory, we could just examine the x/y coordinates, but
		// there's no way to get at those without reflection hacks.
		//
		// WARNING: If this ever needs to support NIST curves other
		// than P-256, this will need to get significantly more
		// involved.
		b, err := point.MarshalBinary()
		if err != nil {
			return false
		}
		if x, _ := elliptic.Unmarshal(elliptic.P256(), b); x == nil {
			return false
		}
		return true
	case hasSmallOrderAble:
		// Ed25519 point validation (rejects small-order points)
		return !validator.HasSmallOrder()
	default:
		return false
	}
}

// removed func

// Scalar is a scalar.
type Scalar struct {
	inner kyber.Scalar
}

// Inner returns the actual kyber.Scalar.
// removed func

// Point returns the corresponding point.
// removed func

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (s *Scalar) UnmarshalBinary(data []byte) error {
	inner := suite.Scalar()
	if err := inner.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("pvss/s11n: failed to deserialize scalar: %w", err)
	}

	s.inner = inner

	return nil
}

// UnmarshalPEM decodes a PEM marshaled scalar.
// removed func

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (s Scalar) MarshalBinary() ([]byte, error) {
	data, err := s.inner.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("pvss/s11n: failed to serialize scalar: %w", err)
	}

	return data, nil
}

// MarshalPEM encodes a scalar into PEM form.
// removed func

// LoadOrGeneratePEM loads a scalar from a PEM file on disk.  Iff the
// scalar is missing, a new one will be generated, written, and loaded.
// removed func

// removed func

// removed func

// PubVerShare is a public verifiable share (`pvss.PubVerShare`)
type PubVerShare struct {
	V Point `json:"v"` // Encrypted/decrypted share

	C  Scalar `json:"c"`  // Challenge
	R  Scalar `json:"r"`  // Response
	VG Point  `json:"vg"` // Public commitment with respect to base point G
	VH Point  `json:"vh"` // Public commitment with respect to base point H
}

// removed func

// removed func

// removed func

// CommitShare is a commit share.
type CommitShare struct {
	PolyV Point `json:"poly_v"` // Share of the public commitment polynomial
	PubVerShare
}

// removed func

// removed func

// removed func

// removed func
