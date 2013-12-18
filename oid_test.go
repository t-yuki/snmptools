package agentx

import (
	"testing"
)

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

func TestOIDEquals(t *testing.T) {
	var O = NewOID

	type OIDEqualsTest struct {
		i        OID
		j        OID
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

func TestGetOIDRemainder(t *testing.T) {
	var O = NewOID

	type OIDRemainderTest struct {
		full        OID
		root        OID
		expected    OID
		expectError bool
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
