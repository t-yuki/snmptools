// package agentx implements a snmp agent in Go.
//
// This package intends to expose control over read-only snmp variables to the
// Go stack. A light cgo layer is used to interface with libsnmp.
//
// If the agentx master socket is not at /var/agentx/master,
// agentx.MasterSocket must be changed prior to calling Run() for the first
// time.
//
// See https://github.com/Learnosity/agentx/blob/master/README.md for more.
package agentx
