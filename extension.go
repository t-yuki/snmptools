package agentx

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

var (
	NodeNotFound = fmt.Errorf("Could not find a node at this OID")
)

// According to the docs for pass, only these ASN types are valid
var PassPersistTypes = map[AsnType]bool{
	AsnInteger:          true,
	AsnGauge32:          true,
	AsnCounter32:        true,
	AsnTimeTicks:        true,
	AsnIpAddress:        true,
	AsnObjectIdentifier: true,
	AsnOctetString:      true,
}

// Can either be a leaf or a branch (i.e. a series of other leaf/branch nodes)
//
// Either Leaves() or Value() will be null
//
// The order of the leaves is significant: if the MibNode is rooted at oid
// .1.3.6.1.4.1.89999, and its first leaf is a scalar value, that leaf will be
// at .1.3.6.1.4.1.89999.0.
type MibNode interface {
	Leaves() []MibNode
	Value() *MibLeaf
}

// Get a leaf from a MibNode.
//
// The OID values are treated as indices of the leaves.
func GetLeaf(node MibNode, oid OID) MibNode {
	var leaves []MibNode

	if len(oid) == 0 {
		// Can't get something at an empty OID
		return nil

	} else if leaves = node.Leaves(); leaves == nil {
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

// Get the OID and the leaf AFTER the leaf or branch that this OID points to.
func GetNextLeaf(node MibNode, oid OID) (OID, MibNode) {
	// Get whatever is at this OID
	var (
		leaves     []MibNode
		val        *MibLeaf
		newOID     OID
		thisBranch MibNode
	)

	if len(oid) == 0 {
		// An OID can't be empty
		return nil, nil
	}

	if oid[len(oid)-1] == 0 {
		// An OID can't be at .0 for its last value; we return the .1
		oid = oid.Copy()
		oid[len(oid)-1] = 1
	}

	if thisBranch = GetLeaf(node, oid); thisBranch == nil {
		return nil, nil

	} else if leaves = thisBranch.Leaves(); leaves != nil && len(leaves) == 0 {
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
		panic(fmt.Errorf("MibNode is nil for both Leaves() and Value(): %#v", thisBranch))

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
			return nil, nil
		}
	}

	// TODO - ??
	return nil, nil

}

// A leaf of the mib tree - contains a scalar value
//
// Implements the MibNode interface
type MibLeaf struct {
	asnType AsnType
	value   interface{}
}

// Checks that the AsnType is valid; returns BadValType if not.
func NewMibLeaf(asnType AsnType, value interface{}) (leaf *MibLeaf, err error) {
	leaf = new(MibLeaf)
	if _, ok := PassPersistTypes[asnType]; !ok {
		return leaf, BadValType
	}

	leaf.asnType = asnType
	leaf.value = value

	return
}

func (l *MibLeaf) String() string {
	return fmt.Sprintf("MibLeaf{%s, %v}", l.asnType.PrettyString(), l.value)
}

// A branch of the mib tree - contains a series of other nodes
type BranchNode struct {
	leaves []MibNode
}

// Create a new branch node
//
// If leaves is nil, a new leaf list will be created.
func NewBranchNode(leaves ...MibNode) *BranchNode {
	if leaves == nil {
		leaves = make([]MibNode, 0)
	}
	return &BranchNode{leaves}
}

func (node *BranchNode) Leaves() []MibNode {
	return node.leaves
}

func (node *BranchNode) Value() *MibLeaf {
	return nil
}

func (node *BranchNode) AddLeaf(leaf MibNode) {
	node.leaves = append(node.leaves, leaf)
}

type LeafNode struct {
	leaf *MibLeaf
}

func NewLeafNode(leaf *MibLeaf) LeafNode {
	return LeafNode{leaf}
}

func (node LeafNode) String() string {
	return fmt.Sprintf("snmptools.LeafNode{leaf:*%s}", node.leaf.String())
}

func (node LeafNode) Leaves() []MibNode {
	return nil
}

func (node LeafNode) Value() *MibLeaf {
	return node.leaf
}

type passPersistState int

const (
	Wait passPersistState = iota
	Get
	GetNext
	Shutdown
	ErrorState
)

// PassPersistExtension
//
//
type PassPersistExtension struct {
	input        io.Reader
	output       io.Writer
	callback     func() MibNode
	root         OID
	currentState passPersistState

	mibTree MibNode

	lines  chan string
	errors chan error
}

func NewPassPersistExtension(input io.Reader, output io.Writer, callback func() MibNode, root OID) *PassPersistExtension {
	return &PassPersistExtension{
		input:        input,
		output:       output,
		callback:     callback,
		root:         root,
		currentState: Wait,
		mibTree:      nil,
		lines:        make(chan string),
		errors:       make(chan error),
	}
}

// Find this OID, assuming it's a child of the root OID we are registered at
//
// Returns nil if it's not found.
func (ppe *PassPersistExtension) GetLeaf(partial OID) (leaf *MibLeaf, err error) {
	if partial, err = partial.GetRemainder(ppe.root); err != nil {
		return
	}

	// TODO - find this leaf
	return
}

func (ppe *PassPersistExtension) GetLeafAfter(partial OID) (leaf *MibLeaf, err error) {
	if partial, err = partial.GetRemainder(ppe.root); err != nil {
		return
	}

	// TODO - find the leaf AFTER this one..
	return
}

// Wait responding to requests with the given input and output streams.
//
// Whenever the root OID is requested, the callback provided to the initializer
// is called, allowing calling code an opportunity to update the MIB state.
func (ppe *PassPersistExtension) Serve() error {
	var (
		nextState passPersistState
		err       error
		line      string
	)

	// Get the initial mib state
	ppe.mibTree = ppe.callback()

	// Wait up a goroutine to scan the input stream for lines
	go ppe.scanInput()

	for {
		select {
		case err, _ = <-ppe.errors:
			return err
		case line = <-ppe.lines:
			if nextState, err = ppe.handleLine(line); err != nil {
				return err
			} else if nextState == Shutdown {
				return nil
			} else {
				ppe.currentState = nextState
			}
		}
	}

}

func (ppe *PassPersistExtension) scanInput() {
	scanner := bufio.NewScanner(ppe.input)

	// Yield each line
	for scanner.Scan() {
		ppe.lines <- scanner.Text()
	}

	// The above loop escapes once the input stream has EOF
	if err := scanner.Err(); err != nil {
		ppe.errors <- err
	}

	close(ppe.errors)
	close(ppe.lines)
}

// handleLine contains the core protocol handling
func (ppe *PassPersistExtension) handleLine(line string) (passPersistState, error) {
	var (
		oid, partial OID
		err          error
	)

	switch ppe.currentState {
	case Wait:
		switch strings.ToLower(line) {
		case "":
			return Shutdown, nil
		case "ping":
			fmt.Fprintf(ppe.output, "PONG\n")
		case "get":
			return Get, nil
		case "getnext":
			return GetNext, nil
		default:
			// TODO - error?
		}

	case Get:
		var leaf *MibLeaf

		// GET is simple - it must just emit the requested OID
		if oid, err = NewOIDFromString(line); err != nil {
			return ErrorState, err
		}

		if oid.Equals(ppe.root) {
			ppe.mibTree = ppe.callback()
		}

		if partial, err = oid.GetRemainder(ppe.root); err != nil {
			return ErrorState, err
		}

		if leaf, err = ppe.GetLeaf(partial); err != nil && err == NodeNotFound {
			return ErrorState, err
		} else if leaf == nil {
			// No value at this node - bail out
			fmt.Fprintf(ppe.output, "\n")
			return Wait, nil
		} else {
			// Print out this node
			fmt.Fprintf(ppe.output, "%s\n%s\n%s\n", oid, leaf.asnType.PrettyString(), leaf.value)
		}

	case GetNext:
		// If a GETNEXT is issued on an object that does not exist, the agent
		// MUST return the next instance in the MIB tree that does exist
		//
		// If a GETNEXT is issued for an object that does exist, the agent MUST
		// skip this entry and find the next instance in the MIB tree to return
		//
		// If no more MIB objects exist in the MIB tree then an 'End of MIB'
		// exception is returned
		//
		// Over the pass / pass_persist protocol with netsnmp's snmpd, a
		// newline is considered equivalent to 'End Of MIB'
		var leaf *MibLeaf

		if oid, err = NewOIDFromString(line); err != nil {
			return ErrorState, err
		}

		if oid.Equals(ppe.root) {
			ppe.mibTree = ppe.callback()
		}

		if partial, err = oid.GetRemainder(ppe.root); err != nil {
			return ErrorState, err
		}

		if leaf, err = ppe.GetLeafAfter(partial); err != nil {
			return ErrorState, err
		} else if leaf == nil {
			// No value at this node - bail out
			fmt.Fprintf(ppe.output, "\n")
			return Wait, nil
		} else {
			// Print out this node
			fmt.Fprintf(ppe.output, "%s\n%s\n%s\n", oid, leaf.asnType.PrettyString(), leaf.value)
		}

		// TODO - traverse the tree to find this OID and then emit the _next_
		// one
	}

	return Wait, nil

}
