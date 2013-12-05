agentx
======

An [agentx](http://en.wikipedia.org/wiki/AgentX) implementation in Go.

See the [godoc page](http://godoc.org/github.com/Learnosity/agentx) for documentation.

With this package, a Go process can connect to an SNMP master agent and
register itself for certain OIDs. Callbacks are registered to these OIDs,
giving application code the ability to set OID response values.

A thin cgo wrapper is used to link to `libsnmp`. A native Go implementation
would be a lot better, but RFC 2741 is [long](http://tools.ietf.org/html/rfc2741#section-3.1).

It can be used alongside an snmp client like [gosnmp](https://github.com/alouca/gosnmp),
the tools that come with net-snmp or a network managing system like OpenNMS.

Rationale
---------

SNMP is a network monitoring protocol, but it can easily be extended to monitor
at the application level; its data model is very flexible.

Go's niche as a safe, statically linked systems programming language makes it
perfect for lightweight monitoring and reporting tools with minimal impact on a
production environment; [Ground Control](http://jondot.github.io/groundcontrol/)
for Raspberry Pi is a good example.

This package can be used either to implement an SNMP agent embedded within a Go
service, or a dedicated monitoring tool. At Learnosity, our monitoring service
is written in Go and exposes data over HTTP, CLI and SNMP through the agentx
service.

`agentx` is the protocol used to extend SNMP by implementing subagents. This
project links to the C bindings for [net-snmp](http://www.net-snmp.org/), the
main open-source implementation of SNMP.


Configuration
-------------

The value of `agentx.MasterSocket` is the address of the agentx master socket.
This value be changed before Run() is called if the address is different.


In net-snmp, the master agent socket is typically defined in `/etc/snmp/snmpd.conf`:

```
#agentXSocket tcp:localhost:705
agentXSocket /var/agentx/master
```

If this is set to a TCP socket, the agentx server can register with an SNMP
master agent on a different host.

Installing
--------

Use go get:

```bash
$ go get github.com/Learnosity/agentx
```

Or just put `import "github.com/Learnosity/agentx"` in your Go project and use
`go install`.

Aside from Go, you'll need `libsnmp-dev` for the code to build.

Currently only works if cc points to gcc - Clang (on OSX) doesn't seem to
understand some of the linker flags.

Handlers
--------

Currently only two handlers are implemented - `StringHandler` and `IntHandler`.
The former does some work under the covers to ensure that allocated C strings
will not leak.

It's trivial to implement handlers for simple types, but handling tables is
much more difficult with the libsnmp API. For now, you can achieve this by
traversing the data structure in Go and registering particular properties of
your objects at specific OIDs. If anyone has experience working with SNMP
tables I'm keen to get them working.

Usage
-----

More full examples coming soon.

```go
// an example of using the agentx library to implement a simple OID handler
package main

import "github.com/Learnosity/agentx"
import "log"
import "C"

// OID values - our handlers will return these
const (
	stringVar = "hello devil here"
	intVar = 666
)
var (
	stringOID = agentx.NewOID(1, 3, 6, 1, 4, 1, 89990, 1)
	intOID = agentx.NewOID(1, 3, 6, 1, 4, 1, 89990, 2)
)


func main() {

	// Register some OID handlers
	//
	// The callbacks here will be invoked whenever SNMP polls for this OID.
	agentx.AddHandler(agentx.NewStringHandler("my-string-var", stringOID, func(req *C.netsnmp_request_info) (string, error) {
		return stringVar, nil
	})

	agentx.AddHandler(agentx.NewIntHandler("my-string-var", stringOID, func(req *C.netsnmp_request_info) (int, error) {
		return intVar, nil
	})

	// Run the snmp agent
	err := agentX.Run()
	if err != nil {
		log.Fatal("Error running agent: %v", err)
	}
}

```

`agentx.Run()` will block indefinitely - call agentx.Stop() in another
goroutine to shut down the agent.

License
-------

MIT-licensed; see `LICENSE`.
