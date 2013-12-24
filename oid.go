package snmptools

import (
	"fmt"
	"strconv"
	"strings"
)

var (
	// Type errors
	BadValType  = fmt.Errorf("Incorrect type for OID value")
	BadOID      = fmt.Errorf("Could not convert OID from C value")
	OIDNotMatch = fmt.Errorf("OIDS did not match")
)

// An snmp OID is just an array of uint32 values
type OID []uint32

// Create a new OID
func NewOID(num ...uint32) OID {
	return num
}

func (oid OID) Equals(other OID) bool {
	// Make sure the OIDs match
	if len(other) != len(oid) {
		return false
	}
	for i := 0; i < len(oid); i += 1 {
		if oid[i] != other[i] {
			return false
		}
	}
	return true
}

func (oid OID) Copy() OID {
	n := make(OID, len(oid))
	for i, num := range oid {
		n[i] = num
	}
	return n
}

// GetRemainder() takes a root OID and returns a partial OID: the remaining
// segment
func (oid OID) GetRemainder(root OID) (OID, error) {
	var partial OID

	// Make sure the OIDs match
	if len(root) > len(oid) {
		return partial, OIDNotMatch
	} else if len(root) == len(oid) {
		return NewOID(), nil
	}

	for i := 0; i < len(root); i += 1 {
		if root[i] != oid[i] {
			return partial, OIDNotMatch
		}
	}

	return oid[len(root):], nil
}

// Add a partial OID to this root OID, returning a new OID
func (oid OID) Add(partial ...uint32) OID {
	var (
		length = len(oid) + len(partial)
		newOID = make(OID, length)
		i      int
	)

	// Copy in the parts of the root OID
	for i = 0; i < len(oid); i += 1 {
		newOID[i] = oid[i]
	}

	// Copy in the parts of the partial OID
	for ; i < length; i += 1 {
		newOID[i] = partial[i-len(oid)]
	}

	return newOID
}

// Parse an OID from a string
func NewOIDFromString(s string) (OID, error) {
	var (
		err  error
		spl  = strings.Split(s, ".")
		o    OID
		conv int
	)

	if len(spl) == 0 {
		return o, BadOID
	}

	o = make(OID, len(spl)-1)

	for i, val := range spl[1:] {
		if conv, err = strconv.Atoi(val); err != nil {
			return o, BadOID
		} else {
			o[i] = uint32(conv)
		}
	}

	return o, nil

}

// Pretty-print the OID with standard notation (each number dot-prefixed)
//
// e.g.:
//
//   .1.3.6.1.4.1.898889.1.0
func (oid OID) String() string {
	if oid == nil {
		return "<nil>"
	}
	var b = make([]byte, 0)
	for _, num := range oid {
		b = append(b, '.')
		b = append(b, []byte(fmt.Sprintf("%d", num))...)
	}
	return string(b)
}

type AsnType byte

func (t AsnType) PrettyString() string {
	s, ok := asnStrings[t]
	if ok {
		return s
	} else {
		return string(t)
	}
}

// SNMP data types
const (
	AsnInteger          AsnType = 0x02
	AsnBitString        AsnType = 0x03
	AsnOctetString      AsnType = 0x04
	AsnNull             AsnType = 0x05
	AsnObjectIdentifier AsnType = 0x06
	AsnSequence         AsnType = 0x30
	AsnIpAddress        AsnType = 0x40
	AsnCounter32        AsnType = 0x41
	AsnGauge32          AsnType = 0x42
	AsnTimeTicks        AsnType = 0x43
	AsnOpaque           AsnType = 0x44
	AsnNsapAddress      AsnType = 0x45
	AsnCounter64        AsnType = 0x46
	AsnUinteger32       AsnType = 0x47
	AsnNoSuchObject     AsnType = 0x80
	AsnNoSuchInstance   AsnType = 0x81
	AsnGetRequest       AsnType = 0xa0
	AsnGetNextRequest   AsnType = 0xa1
	AsnGetResponse      AsnType = 0xa2
	AsnSetRequest       AsnType = 0xa3
	AsnTrap             AsnType = 0xa4
	AsnGetBulkRequest   AsnType = 0xa5
)

var asnStrings = map[AsnType]string{
	AsnInteger:          "integer",
	AsnGauge32:          "gauge",
	AsnCounter32:        "counter",
	AsnTimeTicks:        "timeticks",
	AsnIpAddress:        "ipaddress",
	AsnObjectIdentifier: "objectid",
	AsnOctetString:      "string",
}
