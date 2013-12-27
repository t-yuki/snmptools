package snmptools

import "fmt"

// SMINode is a node in the SMI tree.
//
// Each node is either a subtree or a leaf.
//
//

// SMINode is a node in the SMI tree.
//
// SNMP MIBs use SMI - the Structure of Management Information - to define the hierarchy of managed objects.
//
// Each node is either a subtree or a leaf.  A leaf is a node embedding a real
// value: an SMILeaf. A subtree is a node with 0 or more children; these
// children may be leaves or further subtrees.
//
// Thus, if the node is a subtree, Children() returns the child nodes and Value() will be nil.
// Conversely, if the node is a leaf, Children() will be nil and Value() returns an SMILeaf.
//
// For subtrees, the order of the children is significant: the indices of the array correspond to the sequential child OIDs.
// For example, if the SMINode is a subtree located at .1.3.6.1.4.1.89999, its first child corresponds to 1.3.6.1.4.1.89999.1
type SMINode interface {
	Value() *SMILeaf
	Children() []SMINode
}

// GetLeaf gets a leaf from an SMINode by OID.
//
// The OID is expected to be relative to the node: for example OID(1, 3) will return the third child of the first child of this node.
//
// If the target OID does not match the structure of the node, the return value will be nil.
func GetLeaf(node SMINode, oid OID) SMINode {
	//logger.Debug(fmt.Sprintf("GetLeaf was called with %s", oid))
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

// NextLeaf takes a node in an SMI tree and an OID relative to that node, and
// returns the OID of the _next_ leaf.
//
// For example, if called with .1.3.6, where that OID points at a subtreee, it may return .1.3.6.1, a leaf.
// If called with .1.3.6.1, .1.3.6.2 may be returned.
//
// This is useful for implementing GETNEXT with snmp.
func NextLeaf(node SMINode, oid OID) OID {
	//logger.Debug(fmt.Sprintf("Looking for next leaf from %s", oid))

	if len(oid) == 0 {
		// Empty OID - start with .1
		oid = NewOID(1)
	}

	if oid[len(oid)-1] == 0 {
		// This OID ends in a zero, which is really the address of the subtree;
		// replace the last number with a 1
		oid = oid.Copy()
		oid[len(oid)-1] = 1
	}

	// Now try the various ways getting at the OID
	if thisBranch := GetLeaf(node, oid); thisBranch == nil {
		// There's nothing at this OID - return nil
		return nil

	} else if children := thisBranch.Children(); children != nil && len(children) == 0 {
		// This is a subtree, but it doesn't have any children - return nil
		return nil

	} else if children != nil && children[0].Value() != nil {
		// This is a subtree where the first child is a leaf - that'll do
		return oid.Add(1)

	} else if children != nil {
		// Ths first child of this subtree is itself a subtree - make a recursive call
		return oid.Add(NextLeaf(thisBranch, NewOID(1))...)

	} else if val := thisBranch.Value(); val == nil {
		// Bad situation - this is somehow a node that has no children but also no leaf
		// TODO - log this?
		panic(fmt.Errorf("MibNode is nil for both Children() and Value(): %#v", thisBranch))

	} else {
		// This OID actually points directly at a leaf - this means we need to
		// shift horizontally or even vertically to find the next OID in the
		// tree

		// First, copy the OID and iterate the final number, then test
		// whether an object exists there.
		newOID := oid.Copy()
		newOID[len(newOID)-1] += 1

		if newNode := GetLeaf(node, newOID); newNode != nil {
			// Found a horizontally adjacent leaf - return its OID
			return newOID
		}

		// There's nothing horizontally adjacent - we must move horizontally
		// until a leaf is found or the OID is exhausted
		for len(newOID) > 0 {

			// Remove the final number and increment the end
			newOID = newOID[:len(newOID)-1]
			newOID[len(newOID)-1] += 1

			if n := GetLeaf(node, newOID.Add(1)); n != nil {
				// There's something here
				if n.Value() != nil {
					return newOID.Add(1)
				} else if o := NextLeaf(node, newOID.Add(1)); n != nil {
					return o
				}
			}
		}

		// Nothing was found and the OID was exhausted - return nil
		return nil
	}
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
