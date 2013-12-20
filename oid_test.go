package snmptools

import (
	"testing"
)

// Test parsing an OID from a Go string
func TestGetOIDFromString(t *testing.T) {
	var str = ".1.3.6.1.4.1.898889"
	var expected = OID{1, 3, 6, 1, 4, 1, 898889}

	oid, err := NewOIDFromString(str)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if len(oid) != len(expected) {
		t.Error("Bad OID length")
		t.FailNow()
	}

	for i := range oid {
		if oid[i] != expected[i] {
			t.Error("OIDs do not match")
			t.FailNow()
		}
	}
}

// Test OID equality comparisons
func TestOIDEquals(t *testing.T) {
	var O = NewOID

	type OIDEqualsTest struct {
		i, j     OID
		expected bool
	}

	var tests = []OIDEqualsTest{
		{O(1, 2, 3), O(1, 2, 3, 4), false},
		{O(1, 2, 3, 4, 5, 6), O(1, 2, 3), false},
		{O(1, 2, 3, 4, 5), O(1, 2, 3, 4, 5), true},
	}

	for _, test := range tests {
		if test.i.Equals(test.j) != test.expected {
			t.Fail()
		}
		if test.j.Equals(test.i) != test.expected {
			t.Fail()
		}
	}
}

func TestOIDAdd(t *testing.T) {
	var O = NewOID

	type OIDAddTest struct {
		root, partial, expected OID
	}

	var tests = []OIDAddTest{
		{O(1), O(2), O(1, 2)},
		{O(1, 2, 3, 4, 5), O(6, 7), O(1, 2, 3, 4, 5, 6, 7)},
		{O(1, 2, 3, 4, 5), O(), O(1, 2, 3, 4, 5)},
		{O(), O(1, 2, 3, 4, 5), O(1, 2, 3, 4, 5)},
	}

	for _, test := range tests {
		if !test.root.Add(test.partial).Equals(test.expected) {
			t.Errorf("%s with %s added did not amount to %s", test.root, test.partial, test.expected)
			t.Fail()
		}
	}
}

// Test getting the 'remainder' of an OID by comparing an OID with a parent
func TestGetOIDRemainder(t *testing.T) {
	var O = NewOID

	type OIDRemainderTest struct {
		full, root, expected OID
		expectError          bool
	}

	var tests = []OIDRemainderTest{
		{O(1, 2, 3, 4), O(1, 2, 3, 4), O(), false},
		{O(1, 2, 3, 4), O(1, 2, 3, 4, 5, 6), O(), true},
		{O(1, 2, 3, 4, 5, 6), O(1, 2, 3, 4), O(5, 6), false},
	}

	for _, test := range tests {
		rem, err := test.full.GetRemainder(test.root)
		if test.expectError && err == nil {
			t.Error("Should have seen an error")
			t.FailNow()
		} else if !rem.Equals(test.expected) {
			t.Errorf("Did not get expected remainder: wanted %s, got %s", test.expected, rem)
			t.Fail()
		}

	}

}
