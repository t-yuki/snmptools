package snmptools

import (
	"testing"
)

// Test getting the node at an OID in the MIB Tree
func TestGetOIDFromMIBTree(t *testing.T) {
	// Create a branch with some leaves
	var (
		cnt = 11
		O   = NewOID
	)

	// Create a branch
	branch := NewSMISubtree()

	// Create a list of 10 leaves
	// Add each of them to the branch
	for i := 1; i < cnt; i += 1 {
		branch.AddChild(NewLeafNode(NewSMILeaf(AsnInteger, i)))
	}

	// Create a higher level branch
	outerBranch := NewSMISubtree(branch)

	// Now try to get some leaves!
	type branchTest struct {
		target    OID
		expected  int
		expectNil bool
	}

	branchTests := []branchTest{
		{O(1, 1), 1, false},
		{O(1, 2), 2, false},
		{O(1, 3), 3, false},
		{O(1, 4), 4, false},
		{O(1, 5), 5, false},
		// A missing OID
		{O(2, 1), -1, true},
	}

	for _, test := range branchTests {
		// Try to get the leaf
		node := GetLeaf(outerBranch, test.target)
		if node == nil && !test.expectNil {
			t.Errorf("node should not be nil for %s", test.target)
			t.FailNow()
		} else if node == nil {
			continue
		}

		val := node.Value()
		if val == nil && !test.expectNil {
			t.Error("val should not be nil")
			t.FailNow()
		}

		if v, ok := val.value.(int); !ok || v != test.expected {
			t.Errorf("Got the wrong value: got %d, expected %d", v, test.expected)
			t.Fail()
		}
	}
}

// Test getting the NEXT node from an OID in the MIB Tree
func TestGetNextOIDFromMIBTree(t *testing.T) {
	// Create a branch with some leaves
	var (
		cnt = 11
		O   = NewOID
	)

	// Create a branch
	branchOne := NewSMISubtree()
	branchTwo := NewSMISubtree()
	branchThree := NewSMISubtree()
	branchFour := NewSMISubtree()
	branchFive := NewSMISubtree()

	// Create a list of 10 leaves
	// Add each of them to the branch
	for i := 1; i < cnt; i += 1 {
		branchOne.AddChild(NewLeafNode(NewSMILeaf(AsnInteger, i)))
		branchTwo.AddChild(NewLeafNode(NewSMILeaf(AsnInteger, i)))
		branchThree.AddChild(NewLeafNode(NewSMILeaf(AsnInteger, i)))
		branchFour.AddChild(NewLeafNode(NewSMILeaf(AsnInteger, i)))
		branchFive.AddChild(NewLeafNode(NewSMILeaf(AsnInteger, i)))
	}

	// Branches four and five branch will be fiddly - nested a little more
	intermediateBranch := NewSMISubtree()
	intermediateBranch.AddChild(branchFour)
	intermediateBranch.AddChild(branchFive)

	// Create a higher level branch
	outerBranch := NewSMISubtree()
	outerBranch.AddChild(branchOne)
	outerBranch.AddChild(branchTwo)
	outerBranch.AddChild(branchThree)
	outerBranch.AddChild(intermediateBranch)

	// Now try to get some leaves!
	type branchTest struct {
		target      OID
		expectedOID OID
		expectedVal int
		expectNil   bool
	}

	branchTests := []branchTest{
		{O(0), O(1, 1), 1, false},
		{O(1), O(1, 1), 1, false},
		{O(1, 1), O(1, 2), 2, false},
		{O(1, 2), O(1, 3), 3, false},
		{O(1, 3), O(1, 4), 4, false},
		{O(1, 4), O(1, 5), 5, false},
		{O(1, 5), O(1, 6), 6, false},
		{O(2, 1), O(2, 2), 2, false},
		{O(3, 1), O(3, 2), 2, false},
		{O(1, 10), O(2, 1), 1, false},

		{O(3, 10), O(4, 1, 1), 1, false},
		{O(4, 1, 1), O(4, 1, 2), 2, false},
		{O(4, 1, 10), O(4, 2, 1), 1, false},

		{O(), O(1, 1), 1, false},
		// Some missing / invalid ones
		{O(5, 1), nil, -1, true},
	}

	for _, test := range branchTests {
		// Try to get the leaf
		//node, oid := GetNextLeaf(outerBranch, test.target)
		oid, node := GetNextLeaf(outerBranch, test.target)

		// First, check that the OIDs match
		if !oid.Equals(test.expectedOID) && !test.expectNil {
			t.Errorf("Did not get %s from a GETNEXT with %s - got %s", test.expectedOID, test.target, oid)
			t.Fail()
		}

		if test.expectNil && node != nil {
			t.Errorf("Expected nil when querying for %s; got oid %s and val %s", test.target, oid, node)
			t.Fail()
		}

		// Now check that the retrieved value matches
		if !test.expectNil && test.expectedVal != node.Value().value.(int) {
			t.Errorf("Wrong value when looking up %s; got %v, wanted %d", test.expectedOID, node.Value().value, test.expectedVal)
			t.Fail()
		}
	}
}
