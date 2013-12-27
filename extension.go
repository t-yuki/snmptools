package snmptools

import (
	"bufio"
	"fmt"
	"io"
	"log/syslog"
	"strings"
)

var logger *syslog.Writer

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

const (
	waitState passPersistState = iota
	getState
	getNextState
	shutdownState
	errorState
)

var stateStrings = []string{
	"wait",
	"get",
	"getNext",
	"shutdown",
	"error",
}

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
	ppe.update()

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

func (ppe *PassPersistExtension) update() {
	ppe.mibTree = ppe.callback()
	logger.Debug(fmt.Sprintf("Updated mib tree: %s", ppe.mibTree))
}

func (ppe *PassPersistExtension) scanInput() {
	scanner := bufio.NewScanner(ppe.input)

	// Yield each line
	for scanner.Scan() {
		ppe.lines <- scanner.Text()
	}

	// The above loop escapes once the input stream has EOF
	if err := scanner.Err(); err != nil {
	}

	close(ppe.errors)
	close(ppe.lines)
}

// handleLine contains the core protocol handling
func (ppe *PassPersistExtension) handleLine(line string) (passPersistState, error) {
	logger.Debug(fmt.Sprintf("Handling line: %s in %s state", line, ppe.currentState))
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
			ppe.update()
		}

		if partial, err = oid.GetRemainder(ppe.root); err != nil {
			return errorState, err
		}

		// Call either GetLeaf or GetNextLeaf depending on whether we got getState or getNextState
		if ppe.currentState == getState {
			leaf = GetLeaf(ppe.mibTree, partial)

		} else if ppe.currentState == getNextState {
			if oid = NextLeaf(ppe.mibTree, partial); oid == nil {
				break
			} else {
				leaf = GetLeaf(ppe.mibTree, oid)
				// Combine the root OID with the OID we gave to the subtree to get
				// what we'll use for the response
				oid = ppe.root.Add(oid...)
			}
		}

		if leaf == nil || oid == nil {
			fmt.Fprintf(ppe.output, "None\n")
		} else {
			logger.Debug(fmt.Sprintf("Responding to %v request for %s with OID %s, val %s", ppe.currentState, ppe.root.Add(partial...), oid, leaf.Value()))
			fmt.Fprintf(ppe.output, "%s\n%s\n%v\n", oid, leaf.Value().asnType.PrettyString(), leaf.Value().value)
		}

		return waitState, nil

	default:
		// TODO - ??

	}

	return waitState, nil
}

func init() {
	var err error
	if logger, err = syslog.New(syslog.LOG_LOCAL0, "snmptools"); err != nil {
		panic(err)
	}
}
