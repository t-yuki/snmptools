package agentx

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/alouca/gosnmp"
)

// Make sure that GOMAXPROCS always >1
func TestMaxProcs(t *testing.T) {
	if runtime.GOMAXPROCS(-1) < 2 {
		t.Fatal("runtime.GOMAXPROCS must be >= 2")
	}

}

// Test defining a table type, registering a table handler to an OID, running
// the agent, retrieving some values, and retrieving the agent.
func TestRunAgent(t *testing.T) {
	sig := make(chan bool, 1)
	oid := NewOID(1, 3, 6, 1, 4, 1, 898889, 1)
	stroid := NewOID(1, 3, 6, 1, 4, 1, 898889, 2)

	var (
		intval = 10
		strval = "foo"
	)

	h := NewIntHandler("agentx-test-int", oid, func() (int, error) {
		return intval, nil
	})

	hstr := NewStringHandler("agentx-test-str", stroid, func() (string, error) {
		return strval, nil
	})

	Handlers.Add(h)
	Handlers.Add(hstr)

	go func() {
		err := Run()
		if err != nil {
			fmt.Printf("Error calling Run(): %v\n", err)
			t.Fatal(err)
			t.FailNow()
		}
		sig <- true
	}()

	// Make sure that the snmp agent is running; messy way to do it but it works
	time.Sleep(time.Second / 10)

	if !Running() {
		t.Error("Running() should be true.")
	}

	s, err := gosnmp.NewGoSNMP("127.0.0.1", "public", gosnmp.Version2c, 5)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := s.Get(oid.String() + ".0")
	if err != nil {
		t.Fatal(err.Error())
	}
	if resp.Variables[0].Value != intval {
		t.Fatal("Wrong value - expected %s, got %s", intval, resp.Variables[0].Value)
	}

	resp, err = s.Get(stroid.String() + ".0")
	if err != nil {
		t.Fatal(err.Error())
	}
	if strRes := resp.Variables[0].Value.([]byte); string(strRes) != strval {
		t.Fatal("Wrong value - expected %s, got %s", strval, string(strRes))
	}

	// Now test that we can remove handlers
	Handlers.RemoveAll()
	if resp, err = s.Get(stroid.String() + ".0"); err == nil {
		t.Fatalf("Expected error when requesting a missing handler - got response %v", resp)
	}

	// Test adding it again!
	Handlers.Add(hstr)
	resp, err = s.Get(stroid.String() + ".0")
	if err != nil {
		t.Fatal(err.Error())
	}
	if strRes := resp.Variables[0].Value.([]byte); string(strRes) != strval {
		t.Fatal("Wrong value - expected %s, got %s", strval, string(strRes))
	}

	Stop()

	// Wait for the goroutine calling Run() to signal completion
	<-sig
	if Running() {
		t.Error("Running() should be false.")
	}
}
