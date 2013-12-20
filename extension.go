package snmptools

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
// The search only goes horizontally or downward in the tree from the given
// OID: it will not move upward.
//
// For example , if called with .1.3.6.1.9, GetNextLeaf() it will never return .1.3.6.2.0
func GetNextLeaf(node SMINode, oid OID) (OID, SMINode) {
	// Get whatever is at this OID
	var (
		leaves     []SMINode
		val        *SMILeaf
		newOID     OID
		thisBranch SMINode
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
func NewSMILeaf(asnType AsnType, value interface{}) (leaf *SMILeaf, err error) {
	leaf = new(SMILeaf)
	if _, ok := PassPersistTypes[asnType]; !ok {
		return leaf, BadValType
	}

	leaf.asnType = asnType
	leaf.value = value

	return
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

const (
	waitState passPersistState = iota
	getState
	getNextState
	shutdownState
	errorState
)

// PassPersistExtension is a type holding the state of a pass persist connection with snmpd.
//
// This type can be used to run the process as a child of snmpd, talking to it over STDIO.
type PassPersistExtension struct {
	input        io.Reader
	output       io.Writer
	callback     func() SMINode
	root         OID
	currentState passPersistState

	mibTree SMINode

	lines  chan string
	errors chan error
}

// NewPassPersistExtension() creates a PassPersistExtension object for storing
// the state of the pass persist protocol between the current process and the
// snmpd daemon.
func NewPassPersistExtension(input io.Reader, output io.Writer, callback func() SMINode, root OID) *PassPersistExtension {
	return &PassPersistExtension{
		input:        input,
		output:       output,
		callback:     callback,
		root:         root,
		currentState: waitState,
		mibTree:      nil,
		lines:        make(chan string),
		errors:       make(chan error),
	}
}

// Serve() starts communicating with snmpd over STDIO.
//
// Whenever the root OID that we are registered at is requested, the callback
// that was provided to the initialisation function is called, giving
// client code the opportunity to update the SMINode that is being traversed.
func (ppe *PassPersistExtension) Serve() error {
	var (
		nextState passPersistState
		err       error
		line      string
	)

	// Get the initial MIB state
	ppe.mibTree = ppe.callback()

	// Set up a goroutine to scan the input stream for lines
	go ppe.scanInput()

	for {
		// Handle all the lines, or any error that comes up from input
		// handling.
		select {

		case err, _ = <-ppe.errors:
			return err

		case line = <-ppe.lines:
			if nextState, err = ppe.handleLine(line); err != nil {
				return err
			} else if nextState == shutdownState {
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
	case waitState:
		switch strings.ToLower(line) {
		case "":
			return shutdownState, nil
		case "ping":
			fmt.Fprintf(ppe.output, "PONG\n")
		case "get":
			return getState, nil
		case "getnext":
			return getNextState, nil
		default:
			// TODO - error?
		}

	case getState, getNextState:
		var leaf SMINode

		// GET is simple - it must just emit the requested OID
		if oid, err = NewOIDFromString(line); err != nil {
			return errorState, err
		}

		if oid.Equals(ppe.root) {
			// Request is for the root OID - update the MIB tree
			ppe.mibTree = ppe.callback()
		}

		if partial, err = oid.GetRemainder(ppe.root); err != nil {
			return errorState, err
		}

		// Call either GetLeaf or GetNextLeaf depending on whether we got getState or getNextState
		if ppe.currentState == getState {
			leaf = GetLeaf(ppe.mibTree, partial)
		} else if ppe.currentState == getNextState {
			oid, leaf = GetNextLeaf(ppe.mibTree, oid)
		}

		if leaf == nil || oid == nil {
			fmt.Fprintf(ppe.output, "None\n")
		} else {
			fmt.Fprintf(ppe.output, "%s\n%s\n%s\n", oid, leaf.Value().asnType.PrettyString(), leaf.Value().value)
		}

		return waitState, nil

	default:
		// TODO - ??

	}

	return waitState, nil
}
