package agentx

// #include "sitemon_agent.h"
import "C"

import (
	"log"
	"runtime"
	"sync"
	"unsafe"
)

// oid handlers - a map of OIDHandlers to handler name and a lock
//
// The map is wrapped to ensure safe access.
//
// TODO - add some storage for errors
type OIDHandlers struct {
	m map[string]OIDHandler
	*sync.RWMutex
}

// Singleton instance of Handlers - this is needed so that the reentrant
// functions exported to the cgo layer can look up the map.
var Handlers = &OIDHandlers{make(map[string]OIDHandler), new(sync.RWMutex)}

// All() returns a list of the registered OID handlers
func (h *OIDHandlers) All() []OIDHandler {
	h.RLock()
	defer h.RUnlock()
	handlers := make([]OIDHandler, 0)
	for _, handler := range h.m {
		handlers = append(handlers, handler)
	}
	return handlers
}

// Add() registers an OID Handler
//
// This should only be called before Run() is called for the first type.
func (h *OIDHandlers) Add(handler OIDHandler) {
	h.Lock()
	defer h.Unlock()
	h.m[handler.Name()] = handler
}

// GetHandler() gets a handler by name.
//
// Wraps a map access for safety.
func (h *OIDHandlers) Get(name string) (OIDHandler, bool) {
	h.RLock()
	defer h.RUnlock()
	v, ok := h.m[name]
	return v, ok
}

// Remove() removes a registered OID handler
//
// Returns the removed handler (may be nil if it was not present).
func (h *OIDHandlers) Remove(name string) OIDHandler {
	h.Lock()
	defer h.Unlock()
	handler := h.m[name]
	delete(h.m, name)
	return handler
}

// RemoveAll() removes all registered OID handlers.
func (h *OIDHandlers) RemoveAll() {
	h.Lock()
	defer h.Unlock()
	for k := range h.m {
		delete(h.m, k)
	}
}

// A type wrapping a C struct - netsnmp_request_info
// TODO - parse this into a Go struct
type RequestInfo *C.netsnmp_request_info

func registerScalar(name string, oid OID) {
	var cname = C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	C.sitemon_register_scalar(cname, &oid.C_ulong()[0], C.int(len(oid)))
}

// An OIDHandler is an interface for associating an OID with a function callback.
type OIDHandler interface {
	// Name() returns the name of this table - used as the index
	Name() string

	// OID() returns the OID that this table is registered at
	OID() OID

	// Callback() is called every time an SNMP request comes in for an OID
	// associated with the root OID this handler is registered to. The
	// registered code can then - for example - make calls to snmp through cgo
	// such as snmp_set_var_typed_value.
	//
	// If Callback() returns an error, it will be logged and snmp will be given
	// SNMP_ERR_GENERR.
	// (TODO - set a special typed var for an error message in this case?)
	Callback(OID, RequestInfo) error

	// Register() registers this handler with the snmp master agent.
	// Most Register() implementations can be very simple
	//
	//   func (h OIDHandler) Register() error {
	//		var cname = C.CString(h.Name())
	//		defer C.free(unsafe.Pointer(cname))
	//
	//		C.sitemon_register_scalar(cname, &h.OID().C_ulong()[0], C.int(len(h.OID())))
	//		return nil
	//   }
	Register() error
}

// IntHandler is an OIDHandler interface implementation for simple int values (i.e. ASN_INTEGER)
type IntHandler struct {
	name string
	oid  OID
	cb   func(OID, RequestInfo) (int, error)
}

// NewIntHandler returns an IntHandler associating an oid with a callback.
func NewIntHandler(name string, oid OID, callback func(OID, RequestInfo) (int, error)) *IntHandler {
	return &IntHandler{name, oid, callback}
}

func (h *IntHandler) Name() string {
	return h.name
}

func (h *IntHandler) OID() OID {
	return h.oid
}

func (h *IntHandler) Callback(oid OID, requests RequestInfo) error {
	v, err := h.cb(oid, requests)
	if err != nil {
		return err
	}

	cv := C.int(v)

	C.snmp_set_var_typed_value(requests.requestvb, AsnInteger.u_char(), unsafe.Pointer(&cv), C.size_t(unsafe.Sizeof(cv)))
	return nil
}

func (h *IntHandler) Register() error {
	registerScalar(h.Name(), h.OID())
	return nil
}

// BooleanHandler is an implementation of the OIDHandler interface for Boolean types.
//
// Boolean is not actually a valid SNMP wire type - instead, we set an
// AsnInteger value,ensure that it's either 0 or 1, and rely on the client and
// the mib to determine that the value is a boolean.
type BooleanHandler struct {
	name string
	oid  OID
	cb   func(OID, RequestInfo) (bool, error)
}

func NewBooleanHandler(name string, oid OID, callback func(OID, RequestInfo) (bool, error)) *BooleanHandler {
	return &BooleanHandler{name, oid, callback}
}

func (h *BooleanHandler) Name() string {
	return h.name
}

func (h *BooleanHandler) OID() OID {
	return h.oid
}

func (h *BooleanHandler) Callback(oid OID, requests RequestInfo) error {
	var res C.int
	v, err := h.cb(oid, requests)
	if err != nil {
		return err
	}

	if v {
		res = C.int(1)
	} else {
		res = C.int(0)
	}

	C.snmp_set_var_typed_value(requests.requestvb, AsnInteger.u_char(), unsafe.Pointer(&res), C.size_t(unsafe.Sizeof(res)))
	return nil
}

func (h *BooleanHandler) Register() error {
	registerScalar(h.Name(), h.OID())
	return nil
}

type StringHandler struct {
	name    string
	oid     OID
	cb      func(OID, RequestInfo) (string, error)
	storage unsafe.Pointer
}

func NewStringHandler(name string, oid OID, callback func(OID, RequestInfo) (string, error)) *StringHandler {
	h := &StringHandler{name, oid, callback, nil}

	// Ensure that the associated storage is freed when this object is
	// GC'ed
	runtime.SetFinalizer(h, func(*StringHandler) {
		log.Printf("Freeing storage of handler for %s (%s)", name, oid.String())
		if h.storage != nil {
			C.free(h.storage)
		}
	})

	return h
}

func (h *StringHandler) Name() string {
	return h.name
}

func (h *StringHandler) OID() OID {
	return h.oid
}

func (h *StringHandler) Callback(oid OID, requests RequestInfo) error {
	v, err := h.cb(oid, requests)
	if err != nil {
		return err
	}

	// Free the old storage
	if h.storage != nil {
		C.free(h.storage)
	}

	// Allocate a CString and set the value for this OID as a pointer to it
	cs := C.CString(v)
	C.snmp_set_var_typed_value(requests.requestvb, AsnOctetString.u_char(), unsafe.Pointer(cs), C.strlen(cs))

	// Store the pointer so that it can be freed later
	h.storage = unsafe.Pointer(cs)

	return nil
}

func (h *StringHandler) Register() error {
	registerScalar(h.Name(), h.OID())
	return nil
}

//export golangReentrantHandler
func golangReentrantHandler(cname *C.char, requests RequestInfo, coid *C.oid, oid_length C.int) int {
	var (
		oid  OID
		err  error
		name = C.GoString(cname)
	)

	if oid, err = NewOIDFromCArray(coid, oid_length); err != nil {
		log.Printf("Could not parse C OID in reuqest for %s", name)
		return C.SNMP_ERR_GENERR
	} else {
		log.Printf("Received snmp GET request for %s (%s)\n", oid, name)
	}

	if h, ok := Handlers.Get(name); ok {

		if err := h.Callback(oid, requests); err != nil {
			log.Printf("Error calling callback: %v", err)
			return C.SNMP_ERR_GENERR
		} else {
			return C.SNMP_ERR_NOERROR
		}

	} else {
		// TODO - is this the right variable for a missing value?
		log.Printf("No handler found for %s", oid)
		return C.SNMP_ERR_NOERROR
	}
}
