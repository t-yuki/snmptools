package snmptools

import "fmt"

// SMINode is a node in the SMI tree.
//
// Each node is either a subtree or a leaf.
//
// If the node is a subtree, Children() returns the child nodes and Value() will be nil.
// Conversely, if the node is a leaf, Children() will be nil and Value() returns an SMILeaf.
//
// For subtrees, the order of the children is significant: the indices of the array correspond to the sequential child OIDs.
// For example, if the SMINode is a subtree located at .1.3.6.1.4.1.89999, its first child corresponds to 1.3.6.1.4.1.89999.1
type SMINode interface {
	Children() []SMINode
	Value() *SMILeaf
}

// GetLeaf gets a leaf from an SMINode by OID.
//
// The OID is expected to be relative to the node: for example OID(1, 3) will return the third child of the first child of this node.
//
// If the target OID does not match the structure of the node, the return value will be nil.
func GetLeaf(node SMINode, oid OID) SMINode {
	logger.Debug(fmt.Sprintf("GetLeaf was called with %s", oid))
	var leaves []SMINode

	if len(oid) == 0 {
		// Can't get something at an empty OID
		return nil

	} else if leaves = node.Children(); leaves == nil {
		// There are no leaves here - either GetLeaf has been called on a leaf
		// or for some reason there is a branch with no leaves
		//
		// Try to return the Value from here, in case there's a leaf.
		return NewLeafNode(node.Value())

	} else if int(oid[0])-1 >= len(leaves) {
		// No OID found - there is not a leaf at this index
		return nil

	} else if len(oid) == 1 {
		// We're at the bottom level - return a single leaf
		return leaves[oid[0]-1]

	} else {
		// We're not at the bottom - keep looking for our target recursively
		return GetLeaf(leaves[oid[0]-1], oid[1:])

	}
}

// GetNextLeaf gets the next leaf AFTER the targeted OID from this subtree.
// For example, if called with .1.3.6.1, the node at .1.3.6.2 may be returned.
//
// The search first tries to move horizontally, but it will move upward when
// the current subtree is exhausted.
//
// For example , if called with .1.3.6.1.9, GetNextLeaf() it will never return .1.3.6.2.0
func GetNextLeaf(node SMINode, oid OID) (OID, SMINode) {
	// Get whatever is at this OID
	logger.Debug(fmt.Sprintf("GetNextLeaf was called with %s", oid))
	var (
		leaves     []SMINode
		val        *SMILeaf
		newOID     OID
		thisBranch SMINode
	)

	if len(oid) == 0 {
		// An OID can't be empty - add a .1
		oid = oid.Copy().Add(NewOID(1))
	}

	if oid[len(oid)-1] == 0 {
		// An OID can't be at .0 for its last value; we return the .1
		oid = oid.Copy()
		oid[len(oid)-1] = 1
	}

	if thisBranch = GetLeaf(node, oid); thisBranch == nil {
		return nil, nil

	} else if leaves = thisBranch.Children(); leaves != nil && len(leaves) == 0 {
		// TODO - this shouldn't happen: leaves is non-nil but zero in length?
		// Why would someone add a branch without adding any leaves to it?
		return nil, nil

	} else if leaves != nil && leaves[0].Value() != nil {
		// We have a true leaf - return it
		newOID = oid.Add(NewOID(1))
		//return newOID, NewLeafNode(leaves[0].Value())
		return newOID, GetLeaf(thisBranch, newOID)

	} else if leaves != nil {
		// We need to recurse down to a true leaf
		return GetNextLeaf(leaves[0], NewOID(1))

	} else if val = thisBranch.Value(); val == nil {
		// This shouldn't happen; how can there be a MibNode where the
		// value and leaves are both nil?
		panic(fmt.Errorf("MibNode is nil for both Children() and Value(): %#v", thisBranch))

	} else {
		// This OID points DIRECTLY at a value - we need to find the next one
		// by moving horizontally. This is most easily done by incrementing the
		// final number.
		newOID = oid.Copy()
		newOID[len(newOID)-1] += 1

		// We call GetLeaf with the node that the function was given, not with
		// the branch we've been looking at.
		if newNode := GetLeaf(node, newOID); newNode != nil {
			return newOID, newNode

		} else {

			for len(newOID) > 0 {

				// Copy off the last value and increment the second-last value
				newOID = newOID[:len(newOID)-1]
				newOID[len(newOID)-1] += 1

				if n := GetLeaf(node, newOID.Add(NewOID(1))); n != nil {
					if n.Value() != nil {
						return newOID.Add(NewOID(1)), n
					} else if o, _ := GetNextLeaf(node, newOID.Add(NewOID(1))); n != nil {
						return o, GetLeaf(node, o)
					}
				}

			}
			return nil, nil
		}
	}

	// TODO - ??
	return nil, nil

}

// SMILeaf is a leaf in the mib tree. It has an ASN.1 type and a value.
//
// The valid AsnTypes are limited to those in the PassPersistTypes variable.
type SMILeaf struct {
	asnType AsnType
	value   interface{}
}

// NewSMILeaf() creates a new SMILeaf. Returns BadValType as the error if the
// type is not vaild.

// NewSMILeaf() creates a new SMILeaf.
//
// Logs a warning if the AsnType type is not valid.
func NewSMILeaf(asnType AsnType, value interface{}) *SMILeaf {
	if _, ok := PassPersistTypes[asnType]; !ok {
		logger.Warning(fmt.Sprintf("AsnType not valid for pass_persist extensions: %v", asnType.PrettyString()))
	}
	return &SMILeaf{asnType, value}
}

func (l *SMILeaf) String() string {
	return fmt.Sprintf("MibLeaf{%s, %v}", l.asnType.PrettyString(), l.value)
}

// SMISubtree is a branch in the mib tree, containing a series of other trees
// or leaves as its children.
//
// Implements the SMINode interface.
type SMISubtree struct {
	leaves []SMINode
}

// Create a new branch node
//
// NewSMISubtree() creates a new SMISubtree, optionally taking a list of initial leaves.
func NewSMISubtree(leaves ...SMINode) *SMISubtree {
	if leaves == nil {
		leaves = make([]SMINode, 0)
	}
	return &SMISubtree{leaves}
}

func (node *SMISubtree) String() string {
	var b = make([]byte, 0)

	b = append(b, []byte("SMISubTree{")...)

	if node.leaves != nil {
		for i, child := range node.leaves {
			if i > 0 {
				b = append(b, []byte(", ")...)
			}

			if child.Children() != nil {
				b = append(b, []byte(child.(*SMISubtree).String())...)
			} else if child.Value() != nil {
				b = append(b, []byte(child.Value().String())...)
			}
		}
	}

	b = append(b, []byte("}")...)

	return string(b)
}

func (node *SMISubtree) Children() []SMINode {
	return node.leaves
}

func (node *SMISubtree) Value() *SMILeaf {
	return nil
}

// AddChild() adds a child leaf or subtree to the SMISubTree.
func (node *SMISubtree) AddChild(leaf SMINode) {
	node.leaves = append(node.leaves, leaf)
}

// LeafNode is a leaf in the mib tree, containing a scalar value.
//
// Implements the SMINode interface.
type LeafNode struct {
	leaf *SMILeaf
}

// NewLeafNode() creates a LeafNode.
func NewLeafNode(leaf *SMILeaf) LeafNode {
	return LeafNode{leaf}
}

func (node LeafNode) String() string {
	return fmt.Sprintf("snmptools.LeafNode{leaf:*%s}", node.leaf.String())
}

func (node LeafNode) Children() []SMINode {
	return nil
}

func (node LeafNode) Value() *SMILeaf {
	return node.leaf
}

// passPersistState encapsulates the various states the pass persist handler can be in.
type passPersistState int

func (s passPersistState) String() string {
	return stateStrings[int(s)]
}
