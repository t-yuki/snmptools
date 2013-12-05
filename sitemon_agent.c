#include <stdlib.h>
#include <stdio.h>

/* include netsnmp dependencies */
#include <net-snmp/net-snmp-config.h>
#include <net-snmp/net-snmp-includes.h>
#include <net-snmp/agent/net-snmp-agent-includes.h>

/* include the functions exported from Go */
#include "_cgo_export.h"

int agent_running;
int keep_running;

/* sitemon_init_agent() - configure the agent */
void sitemon_init_agent(char *sockaddr) {
	/* make us an agentx client */
	netsnmp_ds_set_boolean(NETSNMP_DS_APPLICATION_ID, NETSNMP_DS_AGENT_ROLE, 1);
	netsnmp_ds_set_string(
		NETSNMP_DS_APPLICATION_ID, NETSNMP_DS_AGENT_X_SOCKET, sockaddr);

	/* initialize the agent library */
	init_agent("sitemon");

	/* intialize vacm/usm access control */
	init_vacm_vars();
	init_usmUser();

	/* sitemon will be used to read sitemon.conf files */
	init_snmp("sitemon");
}

/* sitemon_run_agent() - run the agent (blocks) */
void sitemon_run_agent() {
	keep_running = 1;
	agent_running = 1;
	while(keep_running) {
		agent_check_and_process(1);
	}
	agent_running = 0;
}

/* sitemon_stop_agent() - stop the agent */
void sitemon_stop_agent() {
	keep_running = 0;
}

/* sitemon_agent_running() - report whether the agent is running */
int sitemon_agent_running(void) {
	return agent_running;
}

/*
 * sitemon_req_handler() - Handle a polling request from an SNMP client / NMS.
 *
 * The handler name, request info and root OID this handler is associated with
 * are all passed up to the Go layer, which can make a decision about what to
 * do. The return value indicates success or failure.
 *
 * See:
 *
 * http://vachacz.blogspot.com.au/2008/04/first-steps-with-net-snmp-and-custom.html
 * http://www.net-snmp.org/dev/agent/helpers_2agent__handler_8h-source.html
 */
int sitemon_req_handler(
		netsnmp_mib_handler				*handler,
		netsnmp_handler_registration	*reginfo,
		netsnmp_agent_request_info		*reqinfo,
		netsnmp_request_info			*requests) {

	/*
	 * Call the go code which will update the value at the pointer and return
	 * the pointer	
	 */
	switch(reqinfo->mode) {
		case MODE_GET:
			return golangReentrantHandler(reginfo->handlerName, requests, reginfo->rootoid, reginfo->rootoid_len);
		default:
			return SNMP_ERR_GENERR;
	}
}

/*
 * Pretty-print an OID to stdout.
 */
void printOID(oid *sitemon_oid, int len) {
	int i;
	for (i = 0; i < len; i++) {
		printf(".%ld", sitemon_oid[i]);
	}
	printf("\n");
}

/*
 * sitemon_register_scalar() - Register a scalar OID.
 *
 * The type of this OID does not need to be determined at this stage; it can be
 * set when the request handler is called.
 *
 * Whenever a GET request comes in for this OID, the handler will call
 * golangReentrantHandler() with the handler name, giving it an opportunity to
 * look up the appropriate function and make SNMP calls to set the typed value.
 *
 */
void sitemon_register_scalar(char *name, oid *sitemon_oid, int oid_length) {
	printf("Registering scalar OID - %s: ", name);
	printOID(sitemon_oid, oid_length);

	netsnmp_handler_registration *reg = netsnmp_create_handler_registration(
		name, sitemon_req_handler,
		sitemon_oid, oid_length,
		HANDLER_CAN_RWRITE);

	netsnmp_register_scalar(reg);
}
