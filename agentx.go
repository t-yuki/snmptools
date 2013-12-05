package agentx

// #cgo LDFLAGS: -Wl,-Bsymbolic-functions -Wl,-z,relro -Wl,-z,now -L/usr/lib/i386-linux-gnu -lnetsnmpmibs -lsensors -ldl -lnetsnmpagent -lwrap -Wl,-E -lnetsnmp -lrt -lcrypto -lm
// #include "sitemon_agent.h"
import "C"

import (
	"fmt"
	"log"
	"runtime"
	"unsafe"
)

func init() {
	// Make sure GOMAXPROCS is at least 2
	if runtime.GOMAXPROCS(-1) < 2 {
		runtime.GOMAXPROCS(2)
	}
}

var initialized bool

// The socket address of the agentx master; if this needs to be changed, it
// must be done before Run() is called.
var MasterSocket = "/var/agentx/master"

var (
	// Caller errors
	AlreadyRunning = fmt.Errorf("Cannot call Run() when agent is already running.")

	// Type errors
	BadValType = fmt.Errorf("Incorrect type for OID value")
	BadOID     = fmt.Errorf("Could not convert OID from C value")

	// Errors from SNMP
	SNMPERR_FAILURE = fmt.Errorf("SNMPERR_FAILURE")
)

// Initialize the snmp agent in C.
// This configures the SNMP library and sets up the connection to the master agent
//
// snmpsocket is the path to the unix domain socket set up the master snmpd
// agent - typically /var/agentx/master
func init_agent() error {
	s := C.CString(MasterSocket)
	defer C.free(unsafe.Pointer(s))
	C.sitemon_init_agent(s)

	return nil
}

// Run() starts the snmp agent's run loop in C, which will block until Stop()
// is called from another goroutine.
//
// Any handlers that have been registered with the package will be registered
// with snmp here.
//
// The call into the C stack is a blocking call which is locked to a single OS
// thread. The package init() will ensure that GOMAXPROCS is at least 2, so
// that this cannot lock the whole process.
func Run() error {
	log.Printf("Running snmp agent")
	if int(C.sitemon_agent_running()) != 0 {
		return AlreadyRunning
	}

	if !initialized {
		if err := init_agent(); err != nil {
			return err
		}
		initialized = true
	}

	// Make sure that all handlers are registered
	for _, handler := range Handlers.All() {
		handler.Register()
	}

	// Lock the OS thread before entering the C stack - this guarantees that an
	// OS thread will be dedicated to the snmp agent work
	runtime.LockOSThread()
	C.sitemon_run_agent()
	runtime.UnlockOSThread()

	log.Printf("snmp agent has been stopped")
	return nil
}

// Stop() halts the run loop started by a call to snmpAgent.Run(), allowing it
// to return.
func Stop() {
	log.Printf("Stopping snmp agent")
	C.sitemon_stop_agent()
}

// Running() reports whether the agent is currently running
func Running() bool {
	return int(C.sitemon_agent_running()) > 0
}

type AsnType byte

func (t AsnType) u_char() C.u_char {
	return C.u_char(t)

}

// SNMP data types
const (
	AsnInteger          AsnType = 0x02
	AsnBitString        AsnType = 0x03
	AsnOctetString      AsnType = 0x04
	AsnNull             AsnType = 0x05
	AsnObjectIdentifier AsnType = 0x06
	AsnSequence         AsnType = 0x30
	AsnIpAddress        AsnType = 0x40
	AsnCounter32        AsnType = 0x41
	AsnGauge32          AsnType = 0x42
	AsnTimeTicks        AsnType = 0x43
	AsnOpaque           AsnType = 0x44
	AsnNsapAddress      AsnType = 0x45
	AsnCounter64        AsnType = 0x46
	AsnUinteger32       AsnType = 0x47
	AsnNoSuchObject     AsnType = 0x80
	AsnNoSuchInstance   AsnType = 0x81
	AsnGetRequest       AsnType = 0xa0
	AsnGetNextRequest   AsnType = 0xa1
	AsnGetResponse      AsnType = 0xa2
	AsnSetRequest       AsnType = 0xa3
	AsnTrap             AsnType = 0xa4
	AsnGetBulkRequest   AsnType = 0xa5
)
