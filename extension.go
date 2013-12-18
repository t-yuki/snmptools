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

// A node in a mib tree.
// Either the leaves or the value will be nil.
type MibNode interface {
	Leaves() []MibNode
	Value() *MibLeaf
}

type MibLeaf struct {
	asnType AsnType
	value   interface{}
}

type BranchNode struct {
	leaves []MibNode
}

func NewBranchNode(leaves []MibNode) *BranchNode {
	return &BranchNode{leaves}
}

func (node *BranchNode) Leaves() []MibNode {
	return node.leaves
}

func (node *BranchNode) Value() *MibLeaf {
	return nil
}

type LeafNode struct {
	leaf MibLeaf
}

func NewLeafNode(leaf MibLeaf) *LeafNode {
	return &LeafNode{leaf}
}

func (node *LeafNode) Leaves() []MibNode {
	return nil
}

func (node *LeafNode) Value() MibLeaf {
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
