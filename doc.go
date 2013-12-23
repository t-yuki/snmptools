// package snmptools implements various snmp helpers in Go.
//
// The package is focused on allowing Go code to run as an snmp subagent.
//
// Includes:
// Highlights:
//
// * an OID type with various interesting methods
// * an SMI/MIB tree data type with subtrees and leaves
// * an implementation of the pass persist extension (http://www.net-snmp.org/wiki/index.php/Tut:Extending_snmpd_using_shell_scripts) line protocol used by net-snmp's snmpd
//
// This pacakge can be used alongside an snmp client like gosnmp,
// the tools that come with net-snmp or a network managing system like OpenNMS.
//
// See https://github.com/Learnosity/snmptools/blob/master/README.md for more.
package snmptools
