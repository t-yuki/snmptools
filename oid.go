package agentx

// #include "sitemon_agent.h"
import "C"
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"unsafe"
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

// GetRemainder() takes a root OID and returns a partial OID: the remaining segment
func (oid OID) GetRemainder(root OID) (OID, error) {
	var partial OID

	// Make sure the OIDs match
	if len(root) >= len(oid) {
		return partial, OIDNotMatch
	}

	for i := 0; i < len(root); i += 1 {
		if root[i] != oid[i] {
			return partial, OIDNotMatch
		}
	}

	return oid[len(root):], nil
}

// Parse an OID from a string
func NewOIDFromString(s string) (OID, error) {
	var (
		err  error
		spl  = strings.Split(s, ".")
		o    = make(OID, len(spl))
		conv int
	)

	if len(spl) == 0 {
		return o, BadOID
	}

	for i, val := range spl[1:] {
		if conv, err = strconv.Atoi(val); err != nil {
			return o, BadOID
		} else {
			o[i] = uint32(conv)
		}
	}

	return o, nil

}

// Create an OID from the C representation
func NewOIDFromCArray(coid *C.oid, oid_length C.int) (OID, error) {
	// See http://stackoverflow.com/questions/14826319/go-cgo-how-do-you-use-a-c-array-passed-as-a-pointer
	var (
		o      OID
		err    error
		buf    *bytes.Reader
		result = make([]uint32, int(oid_length))
		size   = C.int(unsafe.Sizeof(*coid))
		b      = C.GoBytes(unsafe.Pointer(coid), size*oid_length)
	)

	// Read a single uint32 from the buffer
	// Each OID is 4 little-endian, with 4 bytes of padding between them
	for i := 0; i < int(size*oid_length); i += 8 {
		var out uint32
		buf = bytes.NewReader(b[i : i+8])
		if err = binary.Read(buf, binary.LittleEndian, &out); err != nil {
			return o, BadOID
		}

		// Append the number to the result
		result[i/8] = out
	}

	return NewOID(result...), nil
}

// Pretty-print the OID with standard notation (each number dot-prefixed)
//
// e.g.:
//
//   .1.3.6.1.4.1.898889.1.0
func (oid OID) String() string {
	var b = make([]byte, 0)
	for _, num := range oid {
		b = append(b, '.')
		b = append(b, []byte(fmt.Sprintf("%d", num))...)
	}
	return string(b)
}

// Convert the OID to a C representation
func (oid OID) C_ulong() []C.ulong {
	coid := make([]C.ulong, len(oid))
	for i, num := range oid {
		coid[i] = C.ulong(num)
	}
	return coid
}
