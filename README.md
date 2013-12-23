snmptools
=========

An SNMP utility library written in native Go.

Highlights:

* an OID type with various interesting methods
* an SMI/MIB tree data type with subtrees and leaves
* an implementation of the [pass persist extension](http://www.net-snmp.org/wiki/index.php/Tut:Extending_snmpd_using_shell_scripts) line protocol used by net-snmp's snmpd

See the [godoc page](http://godoc.org/github.com/Learnosity/snmptools) for documentation.

This pacakge can be used alongside an snmp client like [gosnmp](https://github.com/alouca/gosnmp),
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

Installing
--------

Use go get:

```bash
$ go get github.com/Learnosity/agentx
```

Or just put `import "github.com/Learnosity/agentx"` in your Go project and use
`go install`.

```bash
$ go test
```

Usage
-----

Coming soon.

License
-------

MIT-licensed; see `LICENSE`.
